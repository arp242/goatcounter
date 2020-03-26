// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/errors"
	"zgo.at/utils/sqlutil"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Size stats are stored as a simple day/width with a count.
//  site |    day     | width    | count
// ------+------------+----------+-------
//     1 | 2019-11-30 | 380      |     1
//     1 | 2019-11-30 | 1920     |     2
//     1 | 2019-11-30 | 1920     |     4
func updateSizeStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + width + event.
		type gt struct {
			count       int
			countUnique int
			day         string
			event       sqlutil.Bool
			width       int
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			var width int
			if len(h.Size) > 0 {
				// TODO: apply scaling?
				width = int(h.Size[0])
			}

			day := h.CreatedAt.Format("2006-01-02")

			k := fmt.Sprintf("%s%d%t", day, width, h.Event)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.width = width
				v.event = h.Event
				var err error
				v.count, v.countUnique, err = existingSizeStats(ctx, tx, h.Site,
					day, v.width, v.event)
				if err != nil {
					return err
				}
			}

			v.count += 1
			if h.StartedSession {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "size_stats", []string{"site", "day",
			"width", "count", "count_unique", "event"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.width, v.count, v.countUnique, v.event)
		}
		return ins.Finish()
	})
}

func existingSizeStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day string, width int, event sqlutil.Bool,
) (int, int, error) {

	var c []struct {
		Count       int          `db:"count"`
		CountUnique int          `db:"count_unique"`
		Event       sqlutil.Bool `db:"event"`
	}
	err := tx.SelectContext(txctx, &c,
		`select count, count_unique, event from size_stats
		where site=$1 and day=$2 and width=$3 limit 1`,
		siteID, day, width)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from size_stats where
		site=$1 and day=$2 and width=$3 and event=$4`,
		siteID, day, width, event)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

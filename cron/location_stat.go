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

// Location stats are stored as a simple day/location with a count.
//  site |    day     | location | count
// ------+------------+----------+-------
//     1 | 2019-11-30 | ET       |     1
//     1 | 2019-11-30 | GR       |     2
//     1 | 2019-11-30 | MX       |     4
func updateLocationStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + location + event.
		type gt struct {
			count       int
			countUnique int
			day         string
			event       sqlutil.Bool
			location    string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := fmt.Sprintf("%s%s%t", day, h.Location, h.Event)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.location = h.Location
				v.event = h.Event
				var err error
				v.count, v.countUnique, err = existingLocationStats(ctx, tx,
					h.Site, day, v.location, v.event)
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
		ins := bulk.NewInsert(ctx, "location_stats", []string{"site", "day",
			"location", "count", "count_unique", "event"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.location, v.count, v.countUnique, v.event)
		}
		return ins.Finish()
	})
}

func existingLocationStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, location string, event sqlutil.Bool,
) (int, int, error) {

	var c []struct {
		Count       int          `db:"count"`
		CountUnique int          `db:"count_unique"`
		Event       sqlutil.Bool `db:"event"`
	}
	err := tx.SelectContext(txctx, &c,
		`select count, count_unique, event from location_stats
		where site=$1 and day=$2 and location=$3 limit 1`,
		siteID, day, location)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from location_stats where
		site=$1 and day=$2 and location=$3 and event=$4`,
		siteID, day, location, event)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

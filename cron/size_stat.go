// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/pkg/errors"
	"zgo.at/goatcounter"
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
		// Group by day + width.
		type gt struct {
			count int
			day   string
			width int
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

			k := day + strconv.FormatInt(int64(width), 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.width = width
				var err error
				v.count, err = existingSizeStats(ctx, tx, h.Site, day, v.width)
				if err != nil {
					return err
				}
			}

			v.count += 1
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, tx,
			"size_stats", []string{"site", "day", "width", "count"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.width, v.count)
		}
		return ins.Finish()
	})
}

func existingSizeStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day string, width int,
) (int, error) {

	var c int
	err := tx.GetContext(txctx, &c,
		`select count from size_stats where site=$1 and day=$2 and width=$3`,
		siteID, day, width)
	if err != nil && err != sql.ErrNoRows {
		return 0, errors.Wrap(err, "existing")
	}

	if err != sql.ErrNoRows {
		_, err = tx.ExecContext(txctx,
			`delete from size_stats where site=$1 and day=$2 and width=$3`,
			siteID, day, width)
		if err != nil {
			return 0, errors.Wrap(err, "delete")
		}
	}

	return c, nil
}

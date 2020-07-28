// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"fmt"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// TODO: add path_id here too?

func updateSizeStats(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + width.
		type gt struct {
			count       int
			countUnique int
			day         string
			width       int
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			var width int
			if len(h.Size) > 0 {
				width = int(h.Size[0]) // TODO: apply scaling?
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := fmt.Sprintf("%s%d", day, width)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.width = width
				if !isReindex {
					var err error
					v.count, v.countUnique, err = existingSizeStats(ctx, tx, h.Site,
						day, v.width)
					if err != nil {
						return err
					}
				}
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "size_stats", []string{"site_id", "day",
			"width", "count", "count_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.width, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

func existingSizeStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day string, width int,
) (int, int, error) {

	var c []struct {
		Count       int `db:"count"`
		CountUnique int `db:"count_unique"`
	}
	err := tx.SelectContext(txctx, &c, `/* existingSizeStats */
		select count, count_unique from size_stats
		where site_id=$1 and day=$2 and width=$3 limit 1`,
		siteID, day, width)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from size_stats where
		site_id=$1 and day=$2 and width=$3`,
		siteID, day, width)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

func updateSizeStats(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		type gt struct {
			count       int
			countUnique int
			day         string
			width       int
			pathID      int64
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
			k := day + strconv.Itoa(width) + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.width = width
				v.pathID = h.PathID
				if !isReindex {
					var err error
					v.count, v.countUnique, err = existingSizeStats(ctx, tx, h.Site,
						day, v.width, v.pathID)
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
			"path_id", "width", "count", "count_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID, v.width, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

func existingSizeStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day string, width int,
	pathID int64,
) (int, int, error) {

	var c []struct {
		Count       int `db:"count"`
		CountUnique int `db:"count_unique"`
	}
	err := tx.SelectContext(txctx, &c, `/* existingSizeStats */
		select count, count_unique from size_stats
		where site_id=$1 and day=$2 and width=$3 and path_id=$4 limit 1`,
		siteID, day, width, pathID)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from size_stats where
		site_id=$1 and day=$2 and width=$3 and path_id=$4`,
		siteID, day, width, pathID)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

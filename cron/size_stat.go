// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
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
		if cfg.PgSQL {
			ins.OnConflict(`on conflict on constraint "size_stats#site_id#path_id#day#width" do update set
				count        = size_stats.count        + excluded.count,
				count_unique = size_stats.count_unique + excluded.count_unique`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, day, width) do update set
				count        = size_stats.count        + excluded.count,
				count_unique = size_stats.count_unique + excluded.count_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID, v.width, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

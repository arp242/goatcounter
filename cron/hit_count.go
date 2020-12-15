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

func updateHitCounts(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, db zdb.DB) error {
		// Group by day + pathID
		type gt struct {
			total       int
			totalUnique int
			hour        string
			pathID      int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			hour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := hour + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.total == 0 {
				v.hour = hour
				v.pathID = h.PathID
			}

			v.total += 1
			if h.FirstVisit {
				v.totalUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "hit_counts", []string{"site_id", "path_id",
			"hour", "total", "total_unique"})
		if cfg.PgSQL {
			ins.OnConflict(`on conflict on constraint "hit_counts#site_id#path_id#hour" do update set
				total        = hit_counts.total        + excluded.total,
				total_unique = hit_counts.total_unique + excluded.total_unique`)

			_, err := db.ExecContext(ctx, `lock table hit_counts in exclusive mode`)
			if err != nil {
				return err
			}
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, hour) do update set
				total        = hit_counts.total        + excluded.total,
				total_unique = hit_counts.total_unique + excluded.total_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.pathID, v.hour, v.total, v.totalUnique)
		}
		return ins.Finish()
	})
}

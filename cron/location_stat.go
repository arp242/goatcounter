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

func updateLocationStats(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		type gt struct {
			count       int
			countUnique int
			day         string
			location    string
			pathID      int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + h.Location + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.location = h.Location
				v.pathID = h.PathID
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "location_stats", []string{"site_id", "day",
			"path_id", "location", "count", "count_unique"})
		if cfg.PgSQL {
			ins.OnConflict(`on conflict on constraint "location_stats#site_id#path_id#day#location" do update set
				count        = location_stats.count        + excluded.count,
				count_unique = location_stats.count_unique + excluded.count_unique`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, day, location) do update set
				count        = location_stats.count        + excluded.count,
				count_unique = location_stats.count_unique + excluded.count_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID, v.location, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

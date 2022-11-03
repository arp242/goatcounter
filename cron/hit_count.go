// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateHitCounts(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
		// Group by day + pathID
		type gt struct {
			total  int
			hour   string
			pathID int64
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

			if h.FirstVisit {
				v.total += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "hit_counts", []string{"site_id", "path_id",
			"hour", "total"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "hit_counts#site_id#path_id#hour" do update set
				total = hit_counts.total + excluded.total`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, hour) do update set
				total = hit_counts.total + excluded.total`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.pathID, v.hour, v.total)
		}
		return ins.Finish()
	}), "cron.updateHitCounts")
}

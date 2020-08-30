// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

func updateRefCounts(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + path + ref.
		type gt struct {
			total       int
			totalUnique int
			hour        string
			path        string
			ref         string
			refScheme   *string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			hour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := hour + h.Path + h.Ref
			v := grouped[k]
			if v.total == 0 {
				v.hour = hour
				v.path = h.Path
				v.ref = h.Ref
				v.refScheme = h.RefScheme
			}

			v.total += 1
			if h.FirstVisit {
				v.totalUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "ref_counts", []string{"site", "path",
			"ref", "hour", "total", "total_unique", "ref_scheme"})
		if cfg.PgSQL {
			ins.OnConflict(`on conflict on constraint "ref_counts#site#path#ref#hour" do update set
				total = ref_counts.total + excluded.total,
				total_unique = ref_counts.total_unique + excluded.total_unique`)
		} else {
			ins.OnConflict(`on conflict(site, path, ref, hour) do update set
				total = ref_counts.total + excluded.total,
				total_unique = ref_counts.total_unique + excluded.total_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.path, v.ref, v.hour, v.total, v.totalUnique, v.refScheme)
		}
		return ins.Finish()
	})
}

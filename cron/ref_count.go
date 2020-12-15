// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

func updateRefCounts(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, db zdb.DB) error {
		// Group by day + pathID + ref.
		type gt struct {
			total       int
			totalUnique int
			hour        string
			pathID      int64
			ref         string
			refScheme   *string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			hour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := hour + strconv.FormatInt(h.PathID, 10) + h.Ref
			v := grouped[k]
			if v.total == 0 {
				v.hour = hour
				v.pathID = h.PathID
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
		ins := bulk.NewInsert(ctx, "ref_counts", []string{"site_id", "path_id",
			"ref", "hour", "total", "total_unique", "ref_scheme"})
		if zdb.PgSQL(zdb.MustGet(ctx)) {
			ins.OnConflict(`on conflict on constraint "ref_counts#site_id#path_id#ref#hour" do update set
				total        = ref_counts.total        + excluded.total,
				total_unique = ref_counts.total_unique + excluded.total_unique`)

			_, err := db.ExecContext(ctx, `lock table ref_counts in exclusive mode`)
			if err != nil {
				return err
			}
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, ref, hour) do update set
				total        = ref_counts.total        + excluded.total,
				total_unique = ref_counts.total_unique + excluded.total_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.pathID, v.ref, v.hour, v.total, v.totalUnique, v.refScheme)
		}
		return ins.Finish()
	})
}

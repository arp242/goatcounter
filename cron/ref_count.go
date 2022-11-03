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

func updateRefCounts(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
		// Group by day + pathID + ref.
		type gt struct {
			total     int
			hour      string
			pathID    int64
			ref       string
			refScheme *string
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

			if h.FirstVisit {
				v.total += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "ref_counts", []string{"site_id", "path_id",
			"ref", "hour", "total", "ref_scheme"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "ref_counts#site_id#path_id#ref#hour" do update set
				total = ref_counts.total + excluded.total`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, ref, hour) do update set
				total = ref_counts.total + excluded.total`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.pathID, v.ref, v.hour, v.total, v.refScheme)
		}
		return ins.Finish()
	}), "cron.updateRefCounts")
}

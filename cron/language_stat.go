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
	"zgo.at/zstd/ztype"
)

func updateLanguageStats(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			countUnique int
			day         string
			language    string
			pathID      int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + ztype.Deref(h.Language, "") + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.countUnique == 0 {
				v.day = day
				v.language = ztype.Deref(h.Language, "")
				v.pathID = h.PathID
			}

			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "language_stats", []string{"site_id", "day", "path_id", "language", "count_unique"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "language_stats#site_id#path_id#day#language" do update set
				count_unique = language_stats.count_unique + excluded.count_unique`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, day, language) do update set
				count_unique = language_stats.count_unique + excluded.count_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID, v.language, v.countUnique)
		}
		return ins.Finish()
	}), "cron.updateLanguageStats")
}

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
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count    int
			day      string
			language string
			pathID   int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + ztype.Deref(h.Language, "") + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.language = ztype.Deref(h.Language, "")
				v.pathID = h.PathID
			}

			if h.FirstVisit {
				v.count += 1
			}
			grouped[k] = v
		}

		var (
			siteID = goatcounter.MustGetSite(ctx).ID
			ins    = goatcounter.Tables.LanguageStats.Bulk(ctx)
		)
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.language, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateLanguageStats")
}

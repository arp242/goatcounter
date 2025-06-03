package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateHitCounts(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
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

		var (
			siteID = goatcounter.MustGetSite(ctx).ID
			ins    = goatcounter.Tables.HitCounts.Bulk(ctx)
		)
		for _, v := range grouped {
			ins.Values(siteID, v.pathID, v.hour, v.total)
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateHitCounts")
}

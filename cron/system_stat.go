package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateSystemStats(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count    int
			day      string
			systemID int64
			pathID   int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}
			if h.SystemID == 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.FormatInt(h.SystemID, 10) + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.systemID = h.SystemID
				v.pathID = h.PathID
			}

			if h.FirstVisit {
				v.count += 1
			}
			grouped[k] = v
		}

		var (
			siteID = goatcounter.MustGetSite(ctx).ID
			ins    = goatcounter.Tables.SystemStats.Bulk(ctx)
		)
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.systemID, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateSystemStats")
}

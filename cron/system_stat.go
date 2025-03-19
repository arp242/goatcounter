package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateSystemStats(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
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

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "system_stats", []string{"site_id", "day",
			"path_id", "system_id", "count"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "system_stats#site_id#path_id#day#system_id" do update set
				count = system_stats.count + excluded.count`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, day, system_id) do update set
				count = system_stats.count + excluded.count`)
		}

		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.day, v.pathID, v.systemID, v.count)
			}
		}
		return ins.Finish()
	}), "cron.updateSystemStats")
}

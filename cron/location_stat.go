package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateLocationStats(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count    int
			day      string
			location string
			pathID   int64
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

			// Call this for the side-effect of creating the rows in the
			// locations table. Should be the case in almost all codepaths, but
			// just to be sure. This is all cached, so there's very little
			// overhead.
			(&goatcounter.Location{}).ByCode(ctx, h.Location)

			if h.FirstVisit {
				v.count += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "location_stats", []string{"site_id", "day",
			"path_id", "location", "count"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "location_stats#site_id#path_id#day#location" do update set
				count = location_stats.count + excluded.count`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, day, location) do update set
				count = location_stats.count + excluded.count`)
		}

		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.day, v.pathID, v.location, v.count)
			}
		}
		return ins.Finish()
	}), "cron.updateLocationStats")
}

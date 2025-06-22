package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateLocationStats(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count    int
			day      string
			location string
			pathID   goatcounter.PathID
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + h.Location + strconv.Itoa(int(h.PathID))
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

		var (
			siteID = goatcounter.MustGetSite(ctx).ID
			ins    = goatcounter.Tables.LocationStats.Bulk(ctx)
		)
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.location, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateLocationStats")
}

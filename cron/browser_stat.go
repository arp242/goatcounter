package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateBrowserStats(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count     int
			day       string
			browserID goatcounter.BrowserID
			pathID    goatcounter.PathID
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}
			if h.BrowserID == 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.Itoa(int(h.BrowserID)) + strconv.Itoa(int(h.PathID))
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.browserID = h.BrowserID
				v.pathID = h.PathID
			}

			if h.FirstVisit {
				v.count += 1
			}
			grouped[k] = v
		}

		ins, err := goatcounter.Tables.BrowserStats.Bulk(ctx)
		if err != nil {
			return err
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.browserID, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateBrowserStats")
}

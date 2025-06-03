package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
)

func updateCampaignStats(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count      int
			day        string
			campaignID int64
			ref        string
			pathID     int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 || h.CampaignID == nil || *h.CampaignID == 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.FormatInt(*h.CampaignID, 10) + h.Ref + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.campaignID = *h.CampaignID
				v.ref = h.Ref
				v.pathID = h.PathID
			}

			if h.FirstVisit {
				v.count += 1
			}
			grouped[k] = v
		}

		var (
			siteID = goatcounter.MustGetSite(ctx).ID
			ins    = goatcounter.Tables.CampaignStats.Bulk(ctx)
		)
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.campaignID, v.ref, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateCampaignStats")
}

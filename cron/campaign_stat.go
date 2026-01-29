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
			campaignID goatcounter.CampaignID
			ref        string
			pathID     goatcounter.PathID
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 || h.CampaignID == nil || *h.CampaignID == 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.Itoa(int(*h.CampaignID)) + h.Ref + strconv.Itoa(int(h.PathID))
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

		ins, err := goatcounter.Tables.CampaignStats.Bulk(ctx)
		if err != nil {
			return err
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		for _, v := range grouped {
			if v.count > 0 {
				ins.Values(siteID, v.pathID, v.day, v.campaignID, v.ref, v.count)
			}
		}
		return ins.Finish()
	})
	return errors.Wrap(err, "cron.updateCampaignStats")
}

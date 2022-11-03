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
)

func updateCampaignStats(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
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

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "campaign_stats", []string{"site_id", "day",
			"path_id", "campaign_id", "ref", "count"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "campaign_stats#site_id#path_id#campaign_id#ref#day" do update set
				count = campaign_stats.count + excluded.count`)
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, campaign_id, ref, day) do update set
				count = campaign_stats.count + excluded.count`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID, v.campaignID, v.ref, v.count)
		}
		return ins.Finish()
	}), "cron.updateCampaignStats")
}

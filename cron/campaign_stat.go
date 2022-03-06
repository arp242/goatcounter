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
			count       int
			countUnique int
			day         string
			campaignID  int64
			ref         string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 || h.CampaignID == nil || *h.CampaignID == 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.FormatInt(*h.CampaignID, 10) + h.Ref
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.campaignID = *h.CampaignID
				v.ref = h.Ref
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "campaign_stats", []string{"site_id", "day",
			"campaign_id", "ref", "count", "count_unique"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "campaign_stats#site_id#campaign_id#ref#day" do update set
				count        = campaign_stats.count        + excluded.count,
				count_unique = campaign_stats.count_unique + excluded.count_unique`)

			err := zdb.Exec(ctx, `lock table campaign_stats in exclusive mode`)
			if err != nil {
				return err
			}
		} else {
			ins.OnConflict(`on conflict(site_id, campaign_id, ref, day) do update set
				count        = campaign_stats.count        + excluded.count,
				count_unique = campaign_stats.count_unique + excluded.count_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.day, v.campaignID, v.ref, v.count, v.countUnique)
		}
		return ins.Finish()
	}), "cron.updateCampaignStats")
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"
	"sync"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zlog"
)

func updateBrowserStats(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count       int
			countUnique int
			day         string
			browserID   int64
			pathID      int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}
			if h.UserAgentID == nil {
				continue
			}

			if h.BrowserID == 0 {
				h.BrowserID, _ = getUA(ctx, *h.UserAgentID)
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.FormatInt(h.BrowserID, 10) + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.browserID = h.BrowserID
				v.pathID = h.PathID
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "browser_stats", []string{"site_id", "day",
			"path_id", "browser_id", "count", "count_unique"})
		if zdb.PgSQL(ctx) {
			ins.OnConflict(`on conflict on constraint "browser_stats#site_id#path_id#day#browser_id" do update set
				count        = browser_stats.count        + excluded.count,
				count_unique = browser_stats.count_unique + excluded.count_unique`)

			err := zdb.Exec(ctx, `lock table browser_stats in exclusive mode`)
			if err != nil {
				return err
			}
		} else {
			ins.OnConflict(`on conflict(site_id, path_id, day, browser_id) do update set
				count        = browser_stats.count        + excluded.count,
				count_unique = browser_stats.count_unique + excluded.count_unique`)
		}

		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID, v.browserID, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

var (
	userAgentMap map[int64][2]int64
	getUAOnce    sync.Once
)

func getUA(ctx context.Context, uaID int64) (browser, system int64) {
	// Load all the user_agents in memory; this speeds up things quite a bit,
	// and the IDs never change. This is about 4M for 500k rows.
	getUAOnce.Do(func() {
		var ua []struct {
			UserAgentID int64 `db:"user_agent_id"`
			BrowserID   int64 `db:"browser_id"`
			SystemID    int64 `db:"system_id"`
		}
		err := zdb.Select(ctx, &ua,
			`select user_agent_id, browser_id, system_id from user_agents`)
		if err != nil {
			panic(err)
		}

		userAgentMap = make(map[int64][2]int64, len(ua))
		for _, u := range ua {
			userAgentMap[u.UserAgentID] = [2]int64{u.BrowserID, u.SystemID}
		}
	})

	ua, ok := userAgentMap[uaID]
	if !ok {
		var u goatcounter.UserAgent
		err := u.ByID(ctx, uaID)
		if err != nil {
			zlog.Field("uaID", uaID).Error(err)
			return 0, 0
		}
		ua = [2]int64{u.BrowserID, u.SystemID}
		userAgentMap[uaID] = ua
	}

	return ua[0], ua[1]
}

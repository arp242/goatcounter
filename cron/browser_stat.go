// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"

	"zgo.at/gadget"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/errors"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Browser are stored as a count per browser/version per day:
//
//  site |    day     | browser | version | count
// ------+------------+---------+---------+------
//     1 | 2019-12-17 | Chrome  | 38      |    13
//     1 | 2019-12-17 | Chrome  | 77      |     2
//     1 | 2019-12-17 | Opera   | 9       |     1
func updateBrowserStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + browser + event.
		type gt struct {
			count       int
			countUnique int
			day         string
			event       zdb.Bool
			browser     string
			version     string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			browser, version := getBrowser(h.Browser)
			if browser == "" {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := fmt.Sprintf("%s%s%s%t", day, browser, version, h.Event)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.browser = browser
				v.version = version
				v.event = h.Event
				var err error
				v.count, v.countUnique, err = existingBrowserStats(ctx, tx,
					h.Site, day, v.browser, v.version, v.event)
				if err != nil {
					return err
				}
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "browser_stats", []string{"site", "day",
			"browser", "version", "count", "count_unique", "event"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.browser, v.version, v.count, v.countUnique, v.event)
		}
		return ins.Finish()
	})
}

func existingBrowserStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, browser, version string, event zdb.Bool,
) (int, int, error) {

	var c []struct {
		Count       int      `db:"count"`
		CountUnique int      `db:"count_unique"`
		Event       zdb.Bool `db:"event"`
	}
	err := tx.SelectContext(txctx, &c, `/* existingBrowserStats */
		select count, count_unique, event from browser_stats
		where site=$1 and day=$2 and browser=$3 and version=$4 limit 1`,
		siteID, day, browser, version)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from browser_stats where
		site=$1 and day=$2 and browser=$3 and version=$4 and event=$5`,
		siteID, day, browser, version, event)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

func getBrowser(uaHeader string) (string, string) {
	ua := gadget.Parse(uaHeader)
	if ua.BrowserName == "Safari" && ua.BrowserVersion == "" {
		fmt.Println(uaHeader)
	}

	return ua.BrowserName, ua.BrowserVersion
}

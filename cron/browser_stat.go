// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mssola/user_agent"
	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bulk"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

func updateAllBrowserStats(ctx context.Context) error {
	var sites goatcounter.Sites
	err := sites.List(ctx)
	if err != nil {
		return err
	}

	l := zlog.Debug("stat").Module("stat")
	for _, s := range sites {
		err := updateSiteBrowserStat(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "site %d", s.ID)
		}
	}
	l.Since("updateAllBrowserStats")
	return nil
}

/*
- Browser       "Firefox 64", "Internet Explorer 11"
- Platform      "Windows 10", "macOS 10.3", "Android 4.2"
*/

type bstat struct {
	Browser   string    `db:"browser"`
	Count     int       `db:"count"`
	CreatedAt time.Time `db:"created_at"`
}

func updateSiteBrowserStat(ctx context.Context, site goatcounter.Site) error {
	ctx = context.WithValue(ctx, ctxkey.Site, &site)
	db := goatcounter.MustGetDB(ctx)

	// Select everything since last update.
	var last string
	if site.LastStat == nil {
		last = "1970-01-01"
	} else {
		last = site.LastStat.Format("2006-01-02")
	}

	var query string
	query = `
		select
			browser,
			count(browser) as count,
			cast(substr(cast(created_at as varchar), 0, 14) || ':00:00' as timestamp) as created_at
		from browsers
		where
			site=$1 and
			created_at>=$2
		group by browser, substr(cast(created_at as varchar), 0, 14)
		order by count desc`

	var stats []bstat
	last = "1970-01-01"
	err := db.SelectContext(ctx, &stats, query, site.ID, last)
	if err != nil {
		return errors.Wrap(err, "fetch data")
	}

	// List what we already have so we can update them, rather than inserting
	// new.
	var have []string
	err = db.SelectContext(ctx, &have,
		`select day||browser from browser_stats where site=$1`,
		site.ID)
	if err != nil {
		return errors.Wrap(err, "have")
	}

	insBrowser := bulk.NewInsert(ctx, goatcounter.MustGetDB(ctx).(*sqlx.DB),
		"browser_stats", []string{"site", "day", "browser", "version", "count"})
	update := map[string]int{}
	for _, s := range stats {
		ua := user_agent.New(s.Browser)
		browser, version := ua.Browser()
		//os := ua.OSInfo()
		day := s.CreatedAt.Format("2006-01-02")
		key := day + browser + " " + version

		exists := false
		for _, h := range have {
			if h == key {
				exists = true
				break
			}
		}

		if exists {
			update[key] += s.Count
		} else {
			insBrowser.Values(site.ID, day, browser, version, s.Count)
			have = append(have, key)
		}

		// wtf := []string{"(Macintosh;", "Python-urllib", "Mozilla", "Ubuntu"}
		// if sliceutil.InStringSlice(wtf, browser) {
		// 	fmt.Println(browser, "->", s.Browser)
		// }
	}
	err = insBrowser.Finish()
	if err != nil {
		return err
	}

	// TODO: updates everything double!
	// for k, count := range update {
	// 	day := k[:10]
	// 	browser := k[10:]
	// 	_, err := db.ExecContext(ctx, `update browser_stats set count=count+$1
	// 		where site=$2 and day=$3 and browser=$4`, count, site.ID, day, browser)
	// 	if err != nil {
	// 		return errors.Wrap(err, "update existing")
	// 	}
	// }

	return nil
}

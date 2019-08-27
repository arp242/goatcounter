package cron

import (
	"context"
	"fmt"
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
	//start := time.Now().Format("2006-01-02 15:04:05")

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
		`select browser from hit_stats where site=$1`,
		site.ID)
	if err != nil {
		return errors.Wrap(err, "have")
	}

	insBrowser := bulk.NewInsert(ctx, goatcounter.MustGetDB(ctx).(*sqlx.DB),
		"browser_stats", []string{"site", "day", "browser", "count"})
	for _, s := range stats {
		ua := user_agent.New(s.Browser)
		b, v := ua.Browser()
		browser := fmt.Sprintf("%s %s", b, v)
		//os := ua.OSInfo()

		exists := false
		for _, h := range have {
			if h == browser {
				exists = true
				break
			}
		}

		_ = exists // TODO
		insBrowser.Values(site.ID, s.CreatedAt.Format("2006-01-02"), browser, s.Count)
	}
	err = insBrowser.Finish()
	if err != nil {
		return err
	}

	return nil
}

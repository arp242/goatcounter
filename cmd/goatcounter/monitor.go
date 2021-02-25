// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"zgo.at/zdb"
	"zgo.at/zlog"
)

const usageMonitor = `
Check if there have been any pageviews in the last n seconds and issue an error
log if it's 0.

Flags:

  -db          Database connection: "sqlite://<file>" or "postgres://<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite://db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -debug       Modules to debug, comma-separated or 'all' for all modules.

  -period      Check every n seconds. Default: 120.

  -once        Check once only and exit instead of checking every -period
               seconds. The -period flag is still used to select the time range.
               The exit code will be 0 if there's at least one pageview, 1 if
               there's 0 pageviews, and 2 if there's another error.

  -site        Limit the check to just one site; makes the query faster.
`

func monitor() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()
	period := CommandLine.Int("period", 120, "")
	once := CommandLine.Bool("once", false, "")
	site := CommandLine.Int("site", 0, "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 2, err
	}

	zlog.Config.SetDebug(*debug)

	db, err := connectDB(*dbConnect, nil, false, true)
	if err != nil {
		return 2, err
	}
	defer db.Close()
	ctx := zdb.WithDB(context.Background(), db)

	query := `/* monitor */ select count(*) from hits where `
	if *site > 0 {
		query += fmt.Sprintf(`site_id=%d and `, *site)
	}
	if zdb.PgSQL(ctx) {
		query += ` created_at > now() - interval '%d seconds'`
	} else {
		query += ` created_at > datetime(datetime(), '-%d seconds')`
	}

	l := zlog.Module("monitor")
	d := time.Duration(*period) * time.Second
	for {
		l.Debug("check")

		var n int
		err := db.Get(context.Background(), &n, fmt.Sprintf(query, *period))
		if err != nil {
			if *once {
				return 2, err
			}
			l.Error(err)
		}
		if n == 0 {
			l.Errorf("no hits")
		} else {
			l.Printf("%d hits", n)
		}

		if *once {
			if n == 0 {
				return 1, nil
			} else {
				return 0, nil
			}
		}

		time.Sleep(d)
	}
}

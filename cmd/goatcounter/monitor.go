// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"fmt"
	"time"

	"zgo.at/zdb"
	"zgo.at/zli"
	"zgo.at/zlog"
)

const usageMonitor = `
Check if there have been any pageviews in the last n seconds and issue an error
log if it's 0.

Flags:

  -db          Database connection: "sqlite+<file>" or "postgres+<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite+/db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

  -period      Check every n seconds. Default: 120.

  -once        Check once only and exit instead of checking every -period
               seconds. The -period flag is still used to select the time range.
               The exit code will be 0 if there's at least one pageview, 1 if
               there's 0 pageviews, and 2 if there's another error.

  -site        Limit the check to just one site; makes the query faster.
`

func cmdMonitor(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	var (
		dbConnect = f.String("sqlite+db/goatcounter.sqlite3", "db").Pointer()
		debug     = f.String("", "debug").Pointer()
		period    = f.Int(120, "period").Pointer()
		once      = f.Bool(false, "once").Pointer()
		site      = f.Int(0, "site").Pointer()
	)
	err := f.Parse()
	if err != nil {
		return err
	}

	return func(dbConnect, debug string, period, site int, once bool) error {
		zlog.Config.SetDebug(debug)

		db, ctx, err := connectDB(dbConnect, []string{"pending"}, false, false)
		if err != nil {
			return err
		}
		defer db.Close()

		query := `/* monitor */ select count(*) from hits where `
		if site > 0 {
			query += fmt.Sprintf(`site_id=%d and `, site)
		}
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			query += ` created_at > now() - interval '%d seconds'`
		} else {
			query += ` created_at > datetime(datetime(), '-%d seconds')`
		}

		l := zlog.Module("monitor")
		timer := time.NewTicker(time.Duration(period) * time.Second)
		ready <- struct{}{}
		for {
			var n int
			err := db.Get(context.Background(), &n, fmt.Sprintf(query, period))
			if err != nil {
				if once {
					return err
				}
				l.Error(err)
			}
			if n == 0 {
				l.Errorf("no hits in last %d seconds", period)
			} else {
				l.Printf("%d hits in last %d seconds", n, period)
			}

			if once {
				if n == 0 {
					return fmt.Errorf("no hits in last %d seconds", period)
				}
				return nil
			}

			select {
			case <-timer.C:
			case <-stop:
				return nil
			}
		}
	}(*dbConnect, *debug, *period, *site, *once)
}

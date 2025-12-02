package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/zdb"
	"zgo.at/zli"
)

const usageMonitor = `
Check if there have been any pageviews in the last n seconds and issue an error
log if it's 0.

Flags:

  -db          Database connection: "sqlite+<file>" or "postgres+<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite+./db/goatcounter.sqlite3 if that database file exists, or
               sqlite+./goatcounter-data/db.sqlite3 if it doesn't.

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

  -period      Check every n seconds. Default: 120.

  -once        Check once and exit instead of checking every -period seconds.
               The -period flag is still used to select the time range. The
               exit code will be 0 if there's at least one pageview, 1 if
               there's 0 pageviews, and 2 if there's another error.

  -site        Limit the check to just one site; makes the query faster.
`

func cmdMonitor(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	var (
		dbConnect = f.String(defaultDB(), "db").Pointer()
		dbConn    = f.String("16,4", "dbconn").Pointer()
		debug     = f.StringList(nil, "debug")
		period    = f.Int(120, "period").Pointer()
		once      = f.Bool(false, "once").Pointer()
		site      = f.Int(0, "site").Pointer()
	)
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil && !errors.As(err, &zli.ErrUnknownEnv{}) {
		return err
	}

	return func(dbConnect, dbConn string, debug []string, period, site int, once bool) error {
		log.SetDebug(debug)

		db, ctx, err := connectDB(dbConnect, dbConn, []string{"pending"}, false, false)
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

		l := log.Module("monitor")
		timer := time.NewTicker(time.Duration(period) * time.Second)
		ready <- struct{}{}
		for {
			var n int
			err := db.Get(context.Background(), &n, fmt.Sprintf(query, period))
			if err != nil {
				if once {
					return err
				}
				l.Error(ctx, err)
			}
			if n == 0 {
				l.Errorf(ctx, "no hits in last %d seconds", period)
			} else {
				l.Infof(ctx, "%d hits in last %d seconds", n, period)
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
	}(*dbConnect, *dbConn, debug.StringsSplit(","), *period, *site, *once)
}

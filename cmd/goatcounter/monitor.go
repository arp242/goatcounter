// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"fmt"
	"os"
	"time"

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

  -site        Limit the check to just one site; makes the query faster.
`

func monitor() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()
	period := CommandLine.Int("period", 120, "")
	site := CommandLine.Int("site", 0, "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}

	zlog.Config.SetDebug(*debug)

	db, err := connectDB(*dbConnect, nil, false)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	query := `/* monitor */ select count(*) from hits where `
	if *site > 0 {
		query += fmt.Sprintf(`site=%d and `, *site)
	}
	query += ` created_at > now() - interval '%d seconds'`

	l := zlog.Module("monitor")
	d := time.Duration(*period) * time.Second
	for {
		l.Debug("check")

		var n int
		err := db.Get(&n, fmt.Sprintf(query, *period))
		if err != nil {
			l.Error(err)
		}
		if n == 0 {
			l.Errorf("no hits")
		} else {
			l.Printf("%d hits", n)
		}

		time.Sleep(d)
	}
}

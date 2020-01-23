// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

// reindex
const usageReindex = `
GoatCounter keeps several *_stats tables so it's less expensive to generate
charts. These are normally updated automatically in the background. This command
recreates these tables.

This is mostly for upgrades; you shouldn't have to run this in normal usage.

This command may take a while to run on larger sites.

Flags:

  -db            Database connection string. Use "sqlite://<dbfile>" for SQLite,
                 or "postgres://<connect string>" for PostgreSQL
                 Default: sqlite://db/goatcounter.sqlite3

  -debug         Modules to debug, comma-separated or 'all' for all modules.

  -confirm       Skip the 10-second safety check.

  -since         Reindex only statistics since this date instead of all of them;
                 as year-month-day.
`

func reindex() error {
	dbConnect := flagDB()
	debug := flagDebug()
	confirm := flag.Bool("confirm", false, "")
	since := flag.String("since", "", "")
	flag.Parse()

	v := zvalidate.New()
	firstDay := v.Date("-since", *since, "2006-01-02")
	if v.HasErrors() {
		return v
	}

	zlog.Config.SetDebug(*debug)

	db, err := connectDB(*dbConnect, nil)
	if err != nil {
		die(1, usage["reindex"], err.Error())
	}
	defer db.Close()

	// TODO: would be best to signal GoatCounter to not persist anything from
	// memstore instead of telling people to stop GoatCounter.
	// OTOH ... this shouldn't be needed very often.
	fmt.Println("This will reindex all the *_stats tables; it's recommended to stop GoatCounter.")
	fmt.Println("This may take a few minutes depending on your data size/computer speed;")
	fmt.Println("you can use e.g. Varnish or some other proxy to send requests to /count later.")
	if !*confirm {
		fmt.Println("Continuing in 10 seconds; press ^C to abort. Use -confirm to skip this.")
		time.Sleep(10 * time.Second)
	}
	fmt.Println("")

	ctx := zdb.With(context.Background(), db)

	where := ""
	last_stat := "null"
	if since != nil && *since != "" {
		where = fmt.Sprintf(" where day >= '%s'", *since)
		last_stat = fmt.Sprintf("'%s'", *since)
	} else {
		var first string
		err := db.GetContext(ctx, &first, `select created_at from hits order by created_at asc limit 1`)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}

		firstDay, err = time.Parse("2006-01-02", first[:10])
		if err != nil {
			return err
		}
	}

	db.MustExecContext(ctx, `delete from hit_stats`+where)
	db.MustExecContext(ctx, `delete from browser_stats`+where)
	db.MustExecContext(ctx, `delete from location_stats`+where)
	db.MustExecContext(ctx, `update sites set last_stat=`+last_stat)

	// Prefill every day with empty entry.
	var allpaths []struct {
		Site int64
		Path string
	}
	err = zdb.MustGet(ctx).SelectContext(ctx, &allpaths,
		`select site, path from hits group by site, path`)
	if err != nil {
		return err
	}

	// Insert paths.
	now := time.Now().UTC()
	day := firstDay
	for {
		var hits []goatcounter.Hit
		err := db.SelectContext(ctx, &hits, `
			select * from hits where
			created_at >= $1 and created_at <= $2`,
			dayStart(day), dayEnd(day))
		if err != nil {
			return err
		}

		prog(fmt.Sprintf("%s → %d", day.Format("2006-01-02"), len(hits)))

		err = cron.ReindexStats(ctx, hits)
		if err != nil {
			return err
		}

		day = day.Add(24 * time.Hour)
		if day.After(now) {
			break
		}
	}
	fmt.Println("")

	return nil
}

func prog(msg string) {
	fmt.Printf("\r\x1b[0K")
	fmt.Printf(msg)
}

func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

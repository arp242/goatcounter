// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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

  -table         Which tables to reindex: hit_stats, browser_stats,
                 location_stats, ref_stats, or all (default).
`

func reindex() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()
	confirm := CommandLine.Bool("confirm", false, "")
	since := CommandLine.String("since", "", "")
	table := CommandLine.String("table", "all", "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}

	v := zvalidate.New()
	firstDay := v.Date("-since", *since, "2006-01-02")
	v.Include("-table", *table, []string{"hit_stats", "browser_stats",
		"location_stats", "ref_stats", "all"})
	if v.HasErrors() {
		return 1, v
	}

	zlog.Config.SetDebug(*debug)

	db, err := connectDB(*dbConnect, nil, false)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	// TODO: would be best to signal GoatCounter to not persist anything from
	// memstore instead of telling people to stop GoatCounter.
	// OTOH ... this shouldn't be needed very often.
	if *table == "all" {
		fmt.Fprintln(stdout, "This will reindex all the *_stats tables; it's recommended to stop GoatCounter.")
	}
	fmt.Fprintln(stdout, "This may take a few minutes depending on your data size/computer speed;")
	fmt.Fprintln(stdout, "you can use e.g. Varnish or some other proxy to send requests to /count later.")
	if !*confirm {
		fmt.Fprintln(stdout, "Continuing in 10 seconds; press ^C to abort. Use -confirm to skip this.")
		time.Sleep(10 * time.Second)
	}
	fmt.Fprintln(stdout, "")

	ctx := zdb.With(context.Background(), db)

	where := ""
	if since != nil && *since != "" {
		where = fmt.Sprintf(" where day >= '%s'", *since)
	} else {
		var first string
		err := db.GetContext(ctx, &first, `select created_at from hits order by created_at asc limit 1`)
		if err != nil {
			if err == sql.ErrNoRows {
				return 0, nil
			}
			return 1, err
		}

		firstDay, err = time.Parse("2006-01-02", first[:10])
		if err != nil {
			return 1, err
		}
	}

	switch *table {
	case "hit_stats":
		db.MustExecContext(ctx, `delete from hit_stats`+where)
	case "browser_stats":
		db.MustExecContext(ctx, `delete from browser_stats`+where)
	case "location_stats":
		db.MustExecContext(ctx, `delete from location_stats`+where)
	case "ref_stats":
		db.MustExecContext(ctx, `delete from ref_stats`+where)
	case "all":
		db.MustExecContext(ctx, `delete from hit_stats`+where)
		db.MustExecContext(ctx, `delete from browser_stats`+where)
		db.MustExecContext(ctx, `delete from location_stats`+where)
		db.MustExecContext(ctx, `delete from ref_stats`+where)
	}

	// Prefill every day with empty entry.
	var allpaths []struct {
		Site int64
		Path string
	}
	err = zdb.MustGet(ctx).SelectContext(ctx, &allpaths,
		`select site, path from hits group by site, path`)
	if err != nil {
		return 1, err
	}

	// Insert paths.
	now := goatcounter.Now()
	day := firstDay
	for {
		var hits []goatcounter.Hit
		err := db.SelectContext(ctx, &hits, `
			select * from hits where
			created_at >= $1 and created_at <= $2`,
			dayStart(day), dayEnd(day))
		if err != nil {
			return 1, err
		}

		fmt.Fprintf(stdout, "\r\x1b[0K%s → %d", day.Format("2006-01-02"), len(hits))

		err = cron.ReindexStats(ctx, hits, *table)
		if err != nil {
			return 1, err
		}

		day = day.Add(24 * time.Hour)
		if day.After(now) {
			break
		}
	}
	fmt.Fprintln(stdout, "")

	return 0, nil
}

func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

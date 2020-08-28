// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	nnow "github.com/jinzhu/now"
	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

// reindex
const usageReindex = `
GoatCounter keeps several *_stats and *_count tables so it's less expensive to
generate charts. These are normally updated automatically in the background.
This command recreates these tables. This is mostly for upgrades; you shouldn't
have to run this in normal usage.

This command may take a while to run on larger sites.

Avoiding race conditions

    You need to be a little bit careful to avoid race conditions with this. It's
    fine to update older data since goatcounter never writes to it, but updating
    the current day may result in:

    1. GoatCounter reads data from DB, processes it, updates the DB.
    2. In the meanwhile reindex updated the data in the DB.

    Generally speaking, it's best to not run this while GoatCounter is running.

Flags:

  -db          Database connection: "sqlite://<file>" or "postgres://<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite://db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -debug       Modules to debug, comma-separated or 'all' for all modules.

  -pause       Number of seconds to pause after each month, to give the server
               some breathing room on large sites. Default: 0.

  -since       Reindex only statistics since this date instead of all of them;
               as year-month-day in UTC.

  -to          Reindex only statistics up to and including this day; as
               year-month-day in UTC. The default is the current day.

  -table       Which tables to reindex: hit_stats, hit_counts, browser_stats,
               system_stats, location_stats, ref_counts, size_stats, or all
               (default).

  -site        Only reindex this site ID. Default is to reindex all.

  -quiet       Don't print progress.
`

func reindex() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()
	since := CommandLine.String("since", "", "")
	to := CommandLine.String("to", "", "")
	table := CommandLine.String("table", "all", "")
	pause := CommandLine.Int("pause", 0, "")
	quiet := CommandLine.Bool("quiet", false, "")
	var site int64
	CommandLine.Int64Var(&site, "site", 0, "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}

	tables := strings.Split(*table, ",")

	v := zvalidate.New()
	firstDay := v.Date("-since", *since, "2006-01-02")
	lastDay := v.Date("-to", *to, "2006-01-02")

	for _, t := range tables {
		v.Include("-table", t, []string{"hit_stats", "hit_counts",
			"browser_stats", "system_stats", "location_stats",
			"ref_counts", "size_stats", "all"})
	}
	if v.HasErrors() {
		return 1, v
	}

	zlog.Config.SetDebug(*debug)

	db, err := connectDB(*dbConnect, nil, false)
	if err != nil {
		return 2, err
	}
	defer db.Close()
	ctx := zdb.With(context.Background(), db)

	if *since == "" {
		w := ""
		if site > 0 {
			w = fmt.Sprintf(" where site=%d ", site)
		}

		var first string
		err := db.GetContext(ctx, &first, `select created_at from hits `+w+` order by created_at asc limit 1`)
		if err != nil {
			if zdb.ErrNoRows(err) {
				return 0, nil
			}
			return 1, err
		}

		firstDay, err = time.Parse("2006-01-02", first[:10])
		if err != nil {
			return 1, err
		}
	}
	if *to == "" {
		lastDay = time.Now().UTC()
	}

	var sites goatcounter.Sites
	err = sites.UnscopedList(ctx)
	if err != nil {
		return 1, err
	}

	for _, s := range sites {
		if site > 0 && s.ID != site {
			continue
		}
		err := dosite(ctx, s, tables, *pause, firstDay, lastDay, *quiet)
		if err != nil {
			return 1, err
		}
	}

	if !*quiet {
		fmt.Fprintln(stdout, "")
	}
	return 0, nil
}

func dosite(ctx context.Context, site goatcounter.Site, tables []string, pause int, firstDay, lastDay time.Time, quiet bool) error {
	db := zdb.MustGet(ctx).(*sqlx.DB)
	siteID := site.ID

	if firstDay.Before(site.CreatedAt) {
		firstDay = site.CreatedAt
	}

	now := goatcounter.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	months := [][]time.Time{
		{firstDay, nnow.With(firstDay).EndOfMonth()},
	}

	start := nnow.With(nnow.With(firstDay).EndOfMonth().Add(12 * time.Hour)).BeginningOfMonth()
	for {
		if start.After(now) {
			break
		}

		end := nnow.With(start).EndOfMonth()
		if end.After(lastDay) {
			months = append(months, []time.Time{start, lastDay})
			break
		}

		months = append(months, []time.Time{start, end})
		start = nnow.With(end.Add(12 * time.Hour)).BeginningOfMonth()
	}

	var allpaths []struct {
		Site int64
		Path string
	}
	err := db.SelectContext(ctx, &allpaths,
		`select path from hits where site=$1 group by path`, siteID)
	if err != nil {
		return err
	}

	// Insert paths.
	query := `select * from hits where site=$1 and created_at >= $2 and created_at <= $3`

	var pauses time.Duration
	if pause > 0 {
		pauses = time.Duration(pause) * time.Second
	}

	for _, month := range months {
		var hits []goatcounter.Hit
		err := db.SelectContext(ctx, &hits, query, siteID, dayStart(month[0]), dayEnd(month[1]))
		if err != nil {
			return err
		}

		if !quiet {
			fmt.Fprintf(stdout, "\r\x1b[0Ksite %d %s → %d", siteID, month[0].Format("2006-01"), len(hits))
		}

		clearMonth(db, tables, month[0].Format("2006-01"), siteID)

		err = cron.ReindexStats(ctx, hits, tables)
		if err != nil {
			return err
		}

		if pauses > 0 {
			time.Sleep(pauses)
		}
	}

	return nil
}

func clearMonth(db *sqlx.DB, tables []string, month string, siteID int64) {
	ctx := context.Background()

	where := fmt.Sprintf(" where site=%d and cast(day as varchar) like '%s-__'", siteID, month)
	for _, t := range tables {
		switch t {
		case "hit_stats":
			db.MustExecContext(ctx, `delete from hit_stats`+where)
		case "hit_counts":
			db.MustExecContext(ctx, fmt.Sprintf(
				`delete from hit_counts where site=%d and cast(hour as varchar) like '%s %%'`,
				siteID, month))
		case "browser_stats":
			db.MustExecContext(ctx, `delete from browser_stats`+where)
		case "system_stats":
			db.MustExecContext(ctx, `delete from system_stats`+where)
		case "location_stats":
			db.MustExecContext(ctx, `delete from location_stats`+where)
		case "ref_counts":
			db.MustExecContext(ctx, fmt.Sprintf(
				`delete from ref_counts where site=%d and cast(hour as varchar) like '%s %%'`,
				siteID, month))
		case "size_stats":
			db.MustExecContext(ctx, `delete from size_stats`+where)
		case "all":
			db.MustExecContext(ctx, `delete from hit_stats`+where)
			db.MustExecContext(ctx, `delete from browser_stats`+where)
			db.MustExecContext(ctx, `delete from system_stats`+where)
			db.MustExecContext(ctx, `delete from location_stats`+where)
			db.MustExecContext(ctx, `delete from size_stats`+where)
			db.MustExecContext(ctx, fmt.Sprintf(
				`delete from hit_counts where site=%d and cast(hour as varchar) like '%s %%'`,
				siteID, month))
			db.MustExecContext(ctx, fmt.Sprintf(
				`delete from ref_counts where site=%d and cast(hour as varchar) like '%s %%'`,
				siteID, month))
		}
	}
}

func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

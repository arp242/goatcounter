// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Commands gcbench inserts random data in a GoatCounter installation for
// performance testing purposes.
package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/zdb"
	"zgo.at/zhttp/ztpl/tplfunc"
	"zgo.at/zli"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zstring"
)

const usage = `gcbench inserts random data in a goatcounter database.

This can be used to give a rough indication of performance expectations.

For some details see: https://github.com/zgoat/goatcounter/blob/master/docs/benchmark.markdown

Flags:

    -tmpdir     Temporary directory to use, for SQLite. Default: /tmp/gcbench.

                Note that /tmp/ is often a tmpfs (stored in memory) and may not
                be indicative of actual performance.

The database is set up though the positional arguments which list the test
profile to run; the format is:

    db engine,nhits,npaths,ndays

        dbengine     Database engine; sqlite or postgres
        nhits        Total amount of pageviews to insert.
        npaths       Spread out over this many paths.
        ndays        And this many days.

For example:

    sqlite,1_000_00,500,365

This will use SQLite, insert 1 million pageviews, spread out over 500 paths, and
365 days.

You can also use an existing database by using a connect flag; for example:

    sqlite:///home/martin/.cache/gcbench/gcbench_20210323T164251_2000_1_1.sqlite3
    postgres://dbname=gcbench_20210323T164251_2000_1_1

This won't insert any pageviews and will just re-run the ports. This assumes the
pageviews are on site_id=1.

Database will *not* be removed after this finishes running as they may be useful
for tweaking or re-running.

Inserting the data is not very fast; especially the SQLite database can take a
*long* time if you want to insert many millions of rows (about 24 hours for 10
million rows on my laptop).
`

type run struct {
	db                      zdb.DB
	ctx                     context.Context
	flag, dbConnect, dbname string
	nHits, nPaths, nDays    int
}

func main() {
	f := zli.NewFlags(os.Args)
	var (
		help   = f.Bool(false, "h", "help")
		tmpdir = f.String("/tmp/gcbench", "tmpdir")
	)
	zli.F(f.Parse())
	if len(f.Args) == 0 || help.Bool() || zstring.Contains(f.Args, "help") {
		fmt.Print(usage)
		return
	}

	zli.F(os.MkdirAll(tmpdir.String(), 0755))

	// Set up all the databases first.
	var runs []run
	n := time.Now()
	for _, a := range f.Args {
		r := run{flag: a}

		dbConnect, h, p, d := zstring.Split4(a, ",")

		gi := func(s, wr string) int {
			i, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 32)
			zli.F(errors.Wrap(err, wr))
			return int(i)
		}
		r.nHits, r.nPaths, r.nDays = gi(h, "nHits"), gi(p, "nPaths"), gi(d, "nDays")

		r.dbname = fmt.Sprintf("gcbench_%s_%d_%d_%d",
			n.Format("20060102T150405"), r.nHits, r.nPaths, r.nDays)

		switch dbConnect {
		default:
			zli.F(errors.New("unknown db: " + dbConnect))
		case "sqlite":
			r.dbname = tmpdir.String() + "/" + r.dbname + ".sqlite3"
			r.dbConnect = "sqlite://" + r.dbname
		case "postgres":
			r.dbConnect = "postgres://dbname=" + r.dbname
		}

		var err error
		r.db, err = zdb.Connect(zdb.ConnectOptions{
			Connect:      r.dbConnect,
			Create:       true,
			GoMigrations: gomig.Migrations,
			Files:        goatcounter.DB,
		})
		zli.F(err)

		first := time.Now().Add(-time.Duration(r.nDays) * 24 * time.Hour)

		r.ctx = goatcounter.NewContext(r.db)
		s := goatcounter.Site{
			Cname:      zstring.NewPtr("gcbench.localhost").P,
			Plan:       goatcounter.PlanBusinessPlus,
			Settings:   goatcounter.SiteSettings{Public: true},
			FirstHitAt: first,
		}
		zli.F(s.Insert(r.ctx))
		r.ctx = goatcounter.WithSite(r.ctx, &s)

		u := goatcounter.User{
			Site:     s.ID,
			Email:    "gcbench@gcbench.localhost",
			Password: []byte("password"),
			Access:   goatcounter.UserAccesses{"all": goatcounter.AccessAdmin},
		}
		zli.F(u.Insert(r.ctx, false))
		r.ctx = goatcounter.WithUser(r.ctx, &u)

		runs = append(runs, r)
	}

	fmt.Println("Using the following databases:")
	for _, r := range runs {
		fmt.Println(zdb.MustGetDB(r.ctx).DriverName(), "\t", r.dbname)
	}
	fmt.Println()

	fmt.Println("Inserting data")
	for _, r := range runs {
		fmt.Print("  ", r.flag)
		err := insert(r)
		if err != nil {
			zli.Errorf("       ERROR inserting data in %s (%s): %s; continuing with the next one",
				r.dbname, r.flag, err)
		}
		zli.ReplaceLinef("  %s \t done\n", r.flag)
		r.db.Close()
	}
	fmt.Println()
}

func connect() {
}

// TODO: bulk inserting data with memstore/cron is pretty darn slow if you want
// to insert 10M pageviews; it's not really designed for these kind of bulk
// imports :-/
//
// See where the time is being spent and if we can speed it up though.
//
// It would be fast if we just bulk-inserted all the pageviews here; but then
// keeping this up to date would be a bit tricky.
func insert(r run) error {
	n := time.Now().UTC()
	dates := make([]time.Time, r.nDays)
	for i := 0; i < r.nDays; i++ {
		dates[i] = time.Date(n.Year(), n.Month(), n.Day()-1-i, 0, 0, 0, 0, time.UTC)
	}

	// TODO: re-use existing paths, user-agents, etc.
	paths := make([]string, r.nPaths)
	for i := range paths {
		paths[i] = "/" + zcrypto.Secret64()
	}
	for i := range ua {
		ua[i] = gadget.Unshorten(ua[i])
	}

	goatcounter.Memstore.Init(zdb.MustGetDB(r.ctx))

	siteID := goatcounter.MustGetSite(r.ctx).ID
	allhits := make([]goatcounter.Hit, 0, 500_000)
	for i := 1; i <= r.nHits; i++ {
		hit := goatcounter.Hit{
			Site:            siteID,
			Path:            paths[rand.Intn(len(paths))],
			CreatedAt:       dates[rand.Intn(len(dates))].Add(time.Duration(rand.Intn(86400)) * time.Second),
			UserAgentHeader: ua[rand.Intn(len(ua))],
			Size:            sizes[rand.Intn(len(sizes))],
		}
		if rand.Intn(10) >= 5 {
			hit.Ref = paths[rand.Intn(len(paths))]
		}

		goatcounter.Memstore.Append(hit)

		if i%1000 == 0 {
			hits, err := goatcounter.Memstore.Persist(r.ctx)
			if err != nil {
				return err
			}
			allhits = append(allhits, hits...)
			zli.ReplaceLinef("  %s \t %d%% %s", r.flag, int(float32(i)/float32(r.nHits)*100), tplfunc.Number(i, ','))
		}

		if len(allhits) >= 500_000 {
			zli.ReplaceLinef("  %s \t %d%% %s  building stat tables…", r.flag, int(float32(i)/float32(r.nHits)*100), tplfunc.Number(i, ','))
			err := cron.ReindexStats(r.ctx, *goatcounter.MustGetSite(r.ctx), allhits, []string{"all"})
			if err != nil {
				return err
			}
			allhits = make([]goatcounter.Hit, 0, 500_000)
		}
	}

	hits, err := goatcounter.Memstore.Persist(r.ctx)
	if err != nil {
		return err
	}
	allhits = append(allhits, hits...)

	zli.ReplaceLinef("  %s \t 100%% %s  building stat tables…", r.flag, tplfunc.Number(r.nHits, ','))
	err = cron.ReindexStats(r.ctx, *goatcounter.MustGetSite(r.ctx), allhits, []string{"all"})
	if err != nil {
		return err
	}
	if zdb.MustGetDB(r.ctx).Driver() == zdb.DriverPostgreSQL {
		return zdb.Exec(r.ctx, "checkpoint")
	}
	return zdb.Exec(r.ctx, "pragma wal_checkpoint")
}

func req(url string) time.Duration {
	s := time.Now()
	b, err := http.Get(url)
	zli.F(err)
	defer b.Body.Close()
	_, err = io.ReadAll(b.Body)
	zli.F(err)
	defer b.Body.Close()
	return time.Since(s).Round(time.Millisecond)
}

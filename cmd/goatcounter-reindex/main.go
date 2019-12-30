// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
)

func main() {
	var (
		confirm bool
		since   string
	)
	flag.BoolVar(&confirm, "confirm", false, "Skip 10-second safety check")
	flag.StringVar(&since, "since", "", "Only reindex days starting with this; as 2006-01-02")
	cfg.Set()

	db, err := zdb.Connect(cfg.DBFile, cfg.PgSQL, nil, nil, "")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// TODO: would be best to signal GoatCounter to not persist anything from
	// memstore instead of telling people to stop GoatCounter.
	// OTOH ... this shouldn't be needed very often.
	fmt.Println("This will reindex all the hit_stats; it's recommended to stop GoatCounter.")
	fmt.Println("This may take a few minutes depending on your data size/computer speed;")
	fmt.Println("you can use e.g. Varnish or some other proxy to send requests to /count later")
	if !confirm {
		fmt.Println("Continuing in 10 seconds; press ^C to abort. Use -confirm to skip this.")
		time.Sleep(10 * time.Second)
	}
	fmt.Println("")

	ctx := zdb.With(context.Background(), db)

	where := ""
	last_stat := "null"
	var firstDay time.Time
	if since != "" {
		firstDay, err = time.Parse("2006-01-02", since)
		if err != nil {
			log.Fatalf("wrong time format for -since: %s", err)
		}

		where = fmt.Sprintf(" where day >= '%s'", since)
		last_stat = fmt.Sprintf("'%s'", since)
	}

	db.MustExecContext(ctx, `delete from hit_stats`+where)
	db.MustExecContext(ctx, `delete from browser_stats`+where)
	db.MustExecContext(ctx, `delete from location_stats`+where)
	db.MustExecContext(ctx, `update sites set last_stat=`+last_stat)

	// Get first hit ever created (start of this GoatCounter site).
	if since == "" {
		var first string
		err = db.GetContext(ctx, &first, `select created_at from hits order by created_at asc limit 1`)
		if err != nil {
			log.Fatal(err)
		}

		firstDay, err = time.Parse("2006-01-02", first[:10])
		if err != nil {
			log.Fatal(err)
		}
	}

	// Prefill every day with empty entry.
	var allpaths []struct {
		Site int64
		Path string
	}
	err = zdb.MustGet(ctx).SelectContext(ctx, &allpaths,
		`select site, path from hits group by site, path`)
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}

		prog(fmt.Sprintf("%s → %d", day.Format("2006-01-02"), len(hits)))

		err = cron.ReindexStats(ctx, hits)
		if err != nil {
			log.Fatal(err)
		}

		day = day.Add(24 * time.Hour)
		if day.After(now) {
			break
		}
	}
	fmt.Println("")
}

func prog(msg string) {
	fmt.Printf("\r\x1b[0K")
	fmt.Printf(msg)
}

func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

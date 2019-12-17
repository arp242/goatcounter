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
	var confirm bool
	flag.BoolVar(&confirm, "confirm", false, "Skip 10-second safety check")
	cfg.Set()

	db, err := zdb.Connect(cfg.DBFile, cfg.PgSQL, nil, nil, "")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("This will reindex all the stats from the hit data.")
	fmt.Println("This may take a long while during which the stats on the site will be wonky.")
	fmt.Println("It's recommended to stop GoatCounter!")
	if !confirm {
		fmt.Println("Continuing in 10 seconds; press ^C to abort. Use -confirm to skip this.")
		time.Sleep(10 * time.Second)
	}
	fmt.Println("")

	ctx := zdb.With(context.Background(), db)

	prog("Deleting stats")
	db.MustExecContext(ctx, `delete from hit_stats`)
	db.MustExecContext(ctx, `delete from browser_stats`)
	db.MustExecContext(ctx, `delete from location_stats`)
	db.MustExecContext(ctx, `update sites set last_stat=null`)
	prog("Stats deleted")

	// Create slice for every day.
	var first string
	err = db.GetContext(ctx, &first, `select created_at from hits order by created_at asc limit 1`)
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().UTC()
	day, err := time.Parse("2006-01-02", first[:10])
	if err != nil {
		log.Fatal(err)
	}

	for {
		prog(fmt.Sprintf("%s", day.Format("2006-01-02")))

		var hits []goatcounter.Hit
		err := db.SelectContext(ctx, &hits, `
			select * from hits where 
			created_at >= $1 and created_at <= $2`,
			dayStart(day), dayEnd(day))
		if err != nil {
			log.Fatal(err)
		}

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

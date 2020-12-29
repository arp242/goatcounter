// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Commands gcbench inserts random data in a GoatCounter installation for
// performance testing purposes.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"zgo.at/gadget"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zli"
	"zgo.at/zstd/zcrypto"
)

const usage = `gcbench inserts random data in a goatcounter database.

Flags with defaults:

    -db=[no default]    Database connection, same format as goatcounter.

    -site=1             SiteID to insert data for.

    -npaths=500         Amount of unique paths to generate.

    -nhits=1_000_00     Total amount of pageviews.

    -ndays=365          Spread out over n days.
`

func main() {
	f := zli.NewFlags(os.Args)
	var (
		help      = f.Bool(false, "h", "help")
		dbConnect = f.String("", "db")
		nPaths    = f.Int(500, "npaths")      // Number of paths
		nHits     = f.Int(1_000_000, "nhits") // Total number of hits.
		nDays     = f.Int(365, "ndays")       // Spread out over n days.
		siteFlag  = f.Int64(1, "site")
	)
	zli.F(f.Parse())

	if help.Bool() {
		fmt.Print(usage)
		return
	}

	db, err := zdb.Connect(zdb.ConnectOptions{
		Connect: dbConnect.String(),
	})
	zli.F(err)
	ctx := zdb.WithDB(context.Background(), db)

	siteID := siteFlag.Int64()
	var site goatcounter.Site
	zli.F(site.ByID(ctx, siteID))

	n := time.Now().UTC()
	dates := make([]time.Time, nDays.Int())
	for i := 0; i < nDays.Int(); i++ {
		dates[i] = time.Date(n.Year(), n.Month(), n.Day()-1-i, 0, 0, 0, 0, time.UTC)
	}

	site.FirstHitAt = dates[len(dates)-1]
	zli.F(site.Update(ctx))

	// TODO: re-use existing paths, user-agents, etc.
	paths := make([]string, nPaths.Int())
	for i := range paths {
		paths[i] = "/" + zcrypto.Secret64()
	}
	for i := range ua {
		ua[i] = gadget.Unshorten(ua[i])
	}

	goatcounter.Memstore.Init(db)

	for i := 0; i <= nHits.Int(); i++ {
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
			zli.F(cron.PersistAndStat(ctx))
			zli.ReplaceLinef("%d/%d", i, nHits.Int())
		}
	}

	zli.F(cron.PersistAndStat(ctx))
	fmt.Println()
}

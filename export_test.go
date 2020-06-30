// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"compress/gzip"
	"os"
	"strings"
	"testing"
	"time"

	"zgo.at/blackmail"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zhttp"
	"zgo.at/zstd/zjson"
	"zgo.at/ztest"
)

func TestExport(t *testing.T) {
	zhttp.InitTpl(nil)
	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)
	ctx, clean := gctest.DB(t)
	defer clean()

	d1 := time.Date(2019, 6, 18, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2019, 6, 19, 0, 0, 0, 0, time.UTC)
	gctest.StoreHits(ctx, t, []goatcounter.Hit{
		{Path: "/asd", CreatedAt: d1},
		{Path: "/zxc", CreatedAt: d1},
		{Path: "/asd", CreatedAt: d2},
	}...)

	var export goatcounter.Export
	defer func() {
		if export.Path != "" {
			os.Remove(export.Path)
		}
	}()
	t.Run("export", func(t *testing.T) {
		fp, err := export.Create(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer fp.Close()

		export.Run(ctx, fp)

		want := strings.ReplaceAll(`{
			"ID": 1,
			"SiteID": 1,
			"StartFromHitID": 0,
			"LastHitID": 3,
			"Path": "/tmp/goatcounter-export-test-%(YEAR)%(MONTH)%(DAY)T%(ANY)Z-0.csv.gz",
			"CreatedAt": "%(YEAR)-%(MONTH)-%(DAY)T%(ANY)Z",
			"FinishedAt": null,
			"NumRows": 3,
			"Size": "0.0",
			"Hash": "sha256-7b756b6dd4d908eff7f7febad0fbdf59f2d7657d8fd09c8ff5133b45f86b1fbf",
			"Error": null
		}`, "\t", "")
		got := string(zjson.MustMarshalIndent(export, "", ""))
		if d := ztest.DiffMatch(got, want); d != "" {
			t.Fatal(d)
		}

		var exports goatcounter.Exports
		err = exports.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(exports) != 1 {
			t.Fatal("exports.List()")
		}
	})

	t.Run("import", func(t *testing.T) {
		fp, err := os.Open(export.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer fp.Close()

		gzfp, err := gzip.NewReader(fp)
		if err != nil {
			t.Fatal(err)
		}
		defer gzfp.Close()

		goatcounter.Import(ctx, gzfp, false, false)

		_, err = goatcounter.Memstore.Persist(ctx)
		if err != nil {
			t.Fatal(err)
		}

		var hits goatcounter.Hits
		_, err = hits.List(ctx, 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 6 {
			t.Fatalf("len(hits) = %d", len(hits))
		}
	})
}

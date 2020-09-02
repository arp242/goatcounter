// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

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
	"zgo.at/zdb"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
)

func TestExport(t *testing.T) {
	ztpl.Init("tpl", nil)
	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)
	ctx, clean := gctest.DB(t)
	defer clean()

	dump := func() string {
		return zdb.DumpString(ctx, `select
			site_id, path, title, event, ua, browser, system, bot, ref, ref_s, size, loc, first, created_at
		from hits_export`)
	}

	d1 := time.Date(2019, 6, 18, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2019, 6, 19, 0, 0, 0, 0, time.UTC)
	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Path: "/asd", CreatedAt: d1, UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/80.0", Title: "Page asd"},
		{Path: "/zxc", CreatedAt: d1, UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/80.0", Title: "Page zxc"},
		{Path: "event", CreatedAt: d2, Event: true},
		{Path: "bot-event", CreatedAt: d2, Event: true, Bot: 1},
		{
			Path:            "/asd",
			CreatedAt:       d2,
			UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0",
			Title:           "Other",
			Location:        "ID",
			Size:            zdb.Floats{1024, 768, 1},
			Ref:             "https://example.com/p",
		},
	}...)

	initial := dump()

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

		export.Run(ctx, fp, false)

		want := strings.ReplaceAll(`{
			"id": 1,
			"site_id": 1,
			"start_from_hit_id": 0,
			"last_hit_id": 5,
			"path": "%(ANY)goatcounter-export-gctest-%(YEAR)%(MONTH)%(DAY)T%(ANY)Z-0.csv.gz",
			"created_at": "%(YEAR)-%(MONTH)-%(DAY)T%(ANY)Z",
			"finished_at": null,
			"num_rows": 5,
			"size": "0.0",
			"hash": "sha256-8a34b87e1ae93292e61984f723fbe6cda8a95dbefc0762ac2864bc98a1a13bc1",
			"error": null
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

		goatcounter.Import(ctx, gzfp, true, false)

		_, err = goatcounter.Memstore.Persist(ctx)
		if err != nil {
			t.Fatal(err)
		}

		out := dump()
		if d := ztest.Diff(out, initial); d != "" {
			t.Error(d)
		}
	})
}

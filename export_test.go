// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"compress/gzip"
	"os"
	"strings"
	"testing"
	"time"

	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
)

func TestExport(t *testing.T) {
	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)
	ctx := gctest.DB(t)

	var site goatcounter.Site
	site.Defaults(ctx)
	site.Code = "gctest2"
	site.Settings.Collect.Set(goatcounter.CollectHits)
	ctx = gctest.Site(ctx, t, &site, nil)
	ctx = goatcounter.WithSite(ctx, &site)

	dump := func() string {
		return zdb.DumpString(ctx, `
		select
			hits.site_id,

			paths.path,
			paths.title,
			paths.event,

			browsers.name || ' ' || browsers.version as browser,
			systems.name  || ' ' || systems.version  as system,

			-- hits.session,
			hits.bot,
			hits.ref,
			hits.ref_scheme as ref_s,
			hits.size,
			hits.location as loc,
			hits.first_visit as first,
			hits.created_at
		from hits
		join paths       using (path_id)
		join browsers    using (browser_id)
		join systems     using (system_id)
		order by hit_id asc`)
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
			Size:            goatcounter.Floats{1024, 768, 1},
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
			"site_id": 2,
			"start_from_hit_id": 0,
			"last_hit_id": 5,
			"path": "%(ANY)goatcounter-export-gctest2-%(YEAR)%(MONTH)%(DAY)T%(ANY)Z-0.csv.gz",
			"created_at": "%(YEAR)-%(MONTH)-%(DAY)T%(ANY)Z",
			"finished_at": null,
			"num_rows": 5,
			"size": "0.1",
			"hash": "sha256-7fb7060000c3e8a1e05bc9f6156fc5571218a234b0a62b4ad6d67a529ad13707",
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

		goatcounter.Import(ctx, gzfp, true, false, func(hit goatcounter.Hit, final bool) {
			if !final {
				goatcounter.Memstore.Append(hit)
			}
		})

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

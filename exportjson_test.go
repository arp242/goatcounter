package goatcounter_test

import (
	"os"
	"testing"

	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
)

func TestJSONExport(t *testing.T) {
	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{FirstVisit: true, Path: "/a", Location: "ID", Size: []float64{1920, 1080, 1},
			UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/80.0"},
		//Hit{FirstVisit: true, Path: "/a", Location: "ID", Size: []float64{1920, 1080, 1},
		//	UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/80.0"},
		//Hit{FirstVisit: true, Path: "/b", Location: "IE", Size: []float64{1920, 1200, 1},
		//	UserAgentHeader: "Chrome/77.0.123.666"},
	)

	var export goatcounter.Export
	defer func() {
		if export.Path != "" {
			os.Remove(export.Path)
		}
	}()

	t.Run("export", func(t *testing.T) {
		fp, err := export.CreateJSON(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer fp.Close()

		export.RunJSON(ctx, fp, false)
	})

	t.Run("import", func(t *testing.T) {
		fp, err := os.Open(export.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer fp.Close()

		var site goatcounter.Site
		site.Defaults(ctx)
		site.Code = "gctest2"
		site.Settings.Collect.Set(goatcounter.CollectHits)
		ctx = gctest.Site(ctx, t, &site, nil)
		ctx = goatcounter.WithSite(ctx, &site)

		_, err = goatcounter.ImportJSON(ctx, fp, true, false)
		if err != nil {
			t.Fatal(err)
		}

		//zdb.Dump(ctx, os.Stderr, `select * from paths`)
		//zdb.Dump(ctx, os.Stderr, `select * from refs`)
		//zdb.Dump(ctx, os.Stderr, `select * from browsers`)
		//zdb.Dump(ctx, os.Stderr, `select * from systems`)
		//zdb.Dump(ctx, os.Stderr, `select * from locations`)
		//zdb.Dump(ctx, os.Stderr, `select * from languages`)
		zdb.Dump(ctx, os.Stderr, `select * from browser_stats`)

		// _, err = goatcounter.Memstore.Persist(ctx)
		// if err != nil {
		// 	t.Fatal(err)
		// }

		// out := dump()
		// if d := ztest.Diff(out, initial); d != "" {
		// 	t.Error(d)
		// }

	})
}

package goatcounter_test

import (
	"archive/zip"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestJSONExport(t *testing.T) {
	ctx := gctest.DB(t)

	gctest.StoreHits(ztime.WithNow(ctx, ztime.FromString("2001-04-05 15:16:17")), t, false,
		Hit{FirstVisit: true, Path: "/a", Title: "AAA", Location: "ID", Size: []float64{1920},
			UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/80.0"},
		Hit{FirstVisit: true, Path: "/a", Title: "AAA", Location: "ID", Size: []float64{1920},
			UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/80.0"},
		Hit{FirstVisit: true, Path: "/b", Title: "BBB", Location: "IE", Size: []float64{1024},
			UserAgentHeader: "Chrome/77.0.123.666", Ref: "http://example.com"},
	)

	gctest.StoreHits(ztime.WithNow(ctx, ztime.FromString("2002-09-05 19:12:13")), t, false,
		Hit{FirstVisit: true, Path: "/c", Title: "CCC",
			UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:80.0) Gecko/20100101 Firefox/150.0"},
	)

	var export, export2 Export
	defer func() {
		if export.Path != "" {
			os.Remove(export.Path)
		}
		if export2.Path != "" {
			os.Remove(export2.Path)
		}
	}()

	{ // Run export.
		fp, err := export.CreateJSON(ctx, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		defer fp.Close()
		export.RunJSON(ctx, fp, false)
		fp.Close()
	}

	testData := func(t *testing.T, firstPathID int) {
		t.Helper()
		haveData := new(bytes.Buffer)
		zdb.Dump(ctx, haveData, `select * from paths`)
		zdb.Dump(ctx, haveData, `select * from refs`)
		zdb.Dump(ctx, haveData, `select * from browsers`)
		zdb.Dump(ctx, haveData, `select * from systems`)
		zdb.Dump(ctx, haveData, `select * from locations`)
		//zdb.Dump(ctx, haveData, `select * from languages`)
		//zdb.Dump(ctx, haveData, `select * from campaigns`)
		wantData := fmt.Sprintf(`
			path_id  site_id  path  title  event
			1        1        /a    AAA    0
			2        1        /b    BBB    0
			3        1        /c    CCC    0
			%d        2        /a    AAA    0
			%d        2        /b    BBB    0
			%d        2        /c    CCC    0

			ref_id  ref          ref_scheme
			1                    o
			2       example.com  h

			browser_id  name     version
			1           Firefox  80
			2           Chrome   77
			3           Firefox  150

			system_id  name   version
			1          Linux
			2

			location_id  iso_3166_2  country  region  country_name  region_name
			1                                         (unknown)
			2            ID          ID               Indonesia
			3            IE          IE               Ireland
		`, firstPathID, firstPathID+1, firstPathID+2)
		if d := ztest.Diff(haveData.String(), wantData, ztest.DiffNormalizeWhitespace); d != "" {
			t.Fatal(d)
		}
	}

	testStats := func(t *testing.T, firstPathID int, dup bool) {
		t.Helper()
		haveStats := new(bytes.Buffer)
		one, two := 1, 2
		if dup {
			one, two = 2, 4
		}
		zdb.Dump(ctx, haveStats, `select * from browser_stats order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from system_stats order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from location_stats order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from size_stats order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from language_stats order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from campaign_stats order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from ref_counts order by path_id`)
		zdb.Dump(ctx, haveStats, `select * from hit_counts order by path_id`)
		wantStats := fmt.Sprintf(`
			site_id  path_id  browser_id  day                  count
			1        1        1           2001-04-05 00:00:00  2
			1        2        2           2001-04-05 00:00:00  1
			1        3        3           2002-09-05 00:00:00  1
			2        %[1]d        1           2001-04-05 00:00:00  %[5]d
			2        %[2]d        2           2001-04-05 00:00:00  %[4]d
			2        %[3]d        3           2002-09-05 00:00:00  %[4]d

			site_id  path_id  system_id  day                  count
			1        1        1          2001-04-05 00:00:00  2
			1        2        2          2001-04-05 00:00:00  1
			1        3        1          2002-09-05 00:00:00  1
			2        %[1]d        1          2001-04-05 00:00:00  %[5]d
			2        %[2]d        2          2001-04-05 00:00:00  %[4]d
			2        %[3]d        1          2002-09-05 00:00:00  %[4]d

			site_id  path_id  day                  location  count
			1        1        2001-04-05 00:00:00  ID        2
			1        2        2001-04-05 00:00:00  IE        1
			1        3        2002-09-05 00:00:00            1
			2        %[1]d        2001-04-05 00:00:00  ID        %[5]d
			2        %[2]d        2001-04-05 00:00:00  IE        %[4]d
			2        %[3]d        2002-09-05 00:00:00            %[4]d

			site_id  path_id  day                  width  count
			1        1        2001-04-05 00:00:00  1920   2
			1        2        2001-04-05 00:00:00  1024   1
			1        3        2002-09-05 00:00:00  0      1
			2        %[1]d        2001-04-05 00:00:00  1920   %[5]d
			2        %[2]d        2001-04-05 00:00:00  1024   %[4]d
			2        %[3]d        2002-09-05 00:00:00  0      %[4]d

			site_id  path_id  day                  language  count
			1        1        2001-04-05 00:00:00            2
			1        2        2001-04-05 00:00:00            1
			1        3        2002-09-05 00:00:00            1
			2        %[1]d        2001-04-05 00:00:00            %[5]d
			2        %[2]d        2001-04-05 00:00:00            %[4]d
			2        %[3]d        2002-09-05 00:00:00            %[4]d

			site_id  path_id  day  campaign_id  ref  count

			site_id  path_id  ref_id  hour                 total
			1        1        1       2001-04-05 15:00:00  2
			1        2        2       2001-04-05 15:00:00  1
			1        3        1       2002-09-05 19:00:00  1
			2        %[1]d        1       2001-04-05 15:00:00  %[5]d
			2        %[2]d        2       2001-04-05 15:00:00  %[4]d
			2        %[3]d        1       2002-09-05 19:00:00  %[4]d

			site_id  path_id  hour                 total
			1        1        2001-04-05 15:00:00  2
			1        2        2001-04-05 15:00:00  1
			1        3        2002-09-05 19:00:00  1
			2        %[1]d        2001-04-05 15:00:00  %[5]d
			2        %[2]d        2001-04-05 15:00:00  %[4]d
			2        %[3]d        2002-09-05 19:00:00  %[4]d
		`, firstPathID, firstPathID+1, firstPathID+2, one, two)
		if d := ztest.Diff(haveStats.String(), wantStats, ztest.DiffNormalizeWhitespace); d != "" {
			t.Fatal(d)
		}
	}

	{ // Run import.
		var site Site
		site.Defaults(ctx)
		site.Code = "gctest2"
		site.Settings.Collect.Set(CollectHits)
		ctx = WithSite(gctest.Site(ctx, t, &site, nil), &site)

		first, err := ImportJSON(ctx, export.Path, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if first == nil || !first.Equal(ztime.FromString("2001-04-05 15:00:00")) {
			t.Fatal(first)
		}
		testData(t, 4)
		testStats(t, 4, false)

		// Run again with replace: should get identical results.
		first, err = ImportJSON(ctx, export.Path, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if first == nil || !first.Equal(ztime.FromString("2001-04-05 15:00:00")) {
			t.Fatal(first)
		}
		testData(t, 7)
		testStats(t, 7, false)

		// Run import again (without replace): data should remain unchanged, but
		// stats should be duplicated.
		first, err = ImportJSON(ctx, export.Path, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if first == nil || !first.Equal(ztime.FromString("2001-04-05 15:00:00")) {
			t.Fatal(first)
		}
		testData(t, 7)
		testStats(t, 7, true)
	}

	{ // Run export with periodStart set.
		fp, err := export2.CreateJSON(ctx, ztime.FromString("2002-09-05 21:00:00"))
		if err != nil {
			t.Fatal(err)
		}
		defer fp.Close()
		export2.RunJSON(ctx, fp, false)
		fp.Close()
	}
	{ // Test export .zip is okay; don't really need to test import here.
		zipf, err := zip.OpenReader(export2.Path)
		if err != nil {
			t.Fatal(err)
		}
		defer zipf.Close()
		slices.SortFunc(zipf.File, func(a, b *zip.File) int { return cmp.Compare(a.Name, b.Name) })
		var have strings.Builder
		for _, f := range zipf.File {
			switch filepath.Base(f.Name) {
			case "info.json", "paths.jsonl", "refs.jsonl", "browsers.jsonl", "systems.jsonl", "locations.jsonl", "languages.jsonl":
				continue
			}
			fp, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(fp)
			if err != nil {
				fp.Close()
				t.Fatal(err)
			}
			fp.Close()
			have.WriteString(filepath.Base(f.Name) + "\n" + string(b) + "\n")
		}

		want := `
			browser_stats.jsonl
			{"day":"2002-09-05","path_id":9,"browser_id":3,"count":2}

			hit_stats.jsonl
			{"hour":"2002-09-05T19:00:00Z","path_id":9,"ref_id":1,"count":2}

			language_stats.jsonl
			{"day":"2002-09-05","path_id":9,"language":"","count":2}

			location_stats.jsonl
			{"day":"2002-09-05","path_id":9,"location":"","count":2}

			size_stats.jsonl
			{"day":"2002-09-05","path_id":9,"width":0,"count":2}

			system_stats.jsonl
			{"day":"2002-09-05","path_id":9,"system_id":1,"count":2}
		`
		if d := ztest.Diff(have.String(), want, ztest.DiffNormalizeWhitespace); d != "" {
			t.Fatal(d)
		}
	}
}

package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestHitStats(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	// Store 3 pageviews for one session: two for "/asd" and one for "/zxc", all
	// on the same time.
	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/asd", Title: "aSd", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Path: "/asd/"}, // Trailing / should be sanitized and treated identical as /asd
		{Site: site.ID, CreatedAt: now, Path: "/zxc"},
	}...)

	check := func(wantT, want0 string) {
		t.Helper()

		var stats goatcounter.HitLists
		display, more, err := stats.List(ctx,
			ztime.NewRange(now.Add(-1*time.Hour)).To(now.Add(1*time.Hour)),
			nil, nil, 10, goatcounter.GroupHourly)
		if err != nil {
			t.Fatal(err)
		}

		gotT := fmt.Sprintf("%d %t", display, more)
		if wantT != gotT {
			t.Fatalf("wrong totals\nhave: %s\nwant: %s", gotT, wantT)
		}
		if len(stats) != 1 {
			t.Fatalf("len(stats) is not 1: %d", len(stats))
		}

		if d := ztest.Diff(string(zjson.MustMarshal(stats[0])), want0, ztest.DiffJSON); d != "" {
			t.Error("first wrong\n" + d)
		}
	}

	check("1 false", `{
			"count": 1,
			"path_id":      1,
			"path":         "/asd",
			"event":        false,
			"title":        "aSd",
			"max":          1,
			"stats": [{
				"day":           "2019-08-31",
				"hourly": [0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],
				"daily":  1
			}]}
		`)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now.Add(2 * time.Hour), Path: "/asd", Title: "aSd", FirstVisit: true},
		{Site: site.ID, CreatedAt: now.Add(2 * time.Hour), Path: "/asd", Title: "aSd"},
	}...)

	check("2 false", `{
			"count":  2,
			"path_id":       1,
			"path":          "/asd",
			"event":         false,
			"title":         "aSd",
			"max":           1,
			"stats":[{
				"day":            "2019-08-31",
				"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,1,0,0,0,0,0,0,0],
				"daily":   2
		}]}`)
}

func TestHitStatsNoCollect(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	site.Settings.Collect ^= goatcounter.CollectSession
	err := site.Update(ctx)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/asd", Title: "aSd"},
		{Site: site.ID, CreatedAt: now, Path: "/asd"},
		{Site: site.ID, CreatedAt: now, Path: "/zxc"},
	}...)

	check := func(wantT, want0, want1 string) {
		t.Helper()

		var stats goatcounter.HitLists
		display, more, err := stats.List(ctx,
			ztime.NewRange(now.Add(-1*time.Hour)).To(now.Add(1*time.Hour)),
			nil, nil, 10, goatcounter.GroupHourly)
		if err != nil {
			t.Fatal(err)
		}

		gotT := fmt.Sprintf("%d %t", display, more)
		if wantT != gotT {
			t.Fatalf("wrong totals\nhave: %s\nwant: %s", gotT, wantT)
		}
		if len(stats) != 2 {
			t.Fatalf("len(stats) is not 2: %d", len(stats))
		}

		if d := ztest.Diff(string(zjson.MustMarshal(stats[0])), want0, ztest.DiffJSON); d != "" {
			t.Error("first wrong\n" + d)
		}

		if d := ztest.Diff(string(zjson.MustMarshal(stats[1])), want1, ztest.DiffJSON); d != "" {
			t.Error("second wrong\n" + d)
		}
	}

	check("3 false", `{
			"count":         2,
			"path_id":       1,
			"path":          "/asd",
			"event":         false,
			"title":         "aSd",
			"max":           2,
			"stats":[{
				"day":            "2019-08-31",
				"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0],
				"daily":   2
		}]}`,
		`{
			"count":         1,
			"path_id":       2,
			"path":          "/zxc",
			"event":         false,
			"title":         "",
			"max":           1,
			"stats":[{
				"day":            "2019-08-31",
				"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],
				"daily":   1
		}]}`,
	)
}

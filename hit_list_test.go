package goatcounter_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestHitListsList(t *testing.T) {
	rng := ztime.NewRange(time.Date(2019, 8, 10, 0, 0, 0, 0, time.UTC)).
		To(time.Date(2019, 8, 17, 23, 59, 59, 0, time.UTC))
	hit := rng.Start.Add(1 * time.Second)

	tests := []struct {
		in         []Hit
		inFilter   string
		inExclude  []PathID
		wantReturn string
		wantStats  HitLists
	}{
		{
			in: []Hit{
				{FirstVisit: true, CreatedAt: hit, Path: "/asd"},
				{FirstVisit: true, CreatedAt: hit.Add(40 * time.Hour), Path: "/asd/"},
				{FirstVisit: true, CreatedAt: hit.Add(100 * time.Hour), Path: "/zxc"},
			},
			wantReturn: "3 false <nil>",
			wantStats: HitLists{
				HitList{Count: 2, Path: "/asd", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(map[int]int{6: 1})},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
				HitList{Count: 1, Path: "/zxc", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(nil)},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(map[int]int{18: 1})},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
		{
			in: []Hit{
				{FirstVisit: true, CreatedAt: hit, Path: "/asd"},
				{FirstVisit: true, CreatedAt: hit, Path: "/zxc"},
			},
			inFilter:   "x",
			wantReturn: "1 false <nil>",
			wantStats: HitLists{
				HitList{Count: 1, Path: "/zxc", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
		{
			in: []Hit{
				{FirstVisit: true, CreatedAt: hit, Path: "/a"},
				{FirstVisit: true, CreatedAt: hit, Path: "/aa"},
				{FirstVisit: true, CreatedAt: hit, Path: "/aaa"},
				{FirstVisit: true, CreatedAt: hit, Path: "/aaaa"},
			},
			inFilter:   "a",
			wantReturn: "2 true <nil>",
			wantStats: HitLists{
				HitList{Count: 1, Path: "/aaaa", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
				HitList{Count: 1, Path: "/aaa", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
		{
			in: []Hit{
				{FirstVisit: true, CreatedAt: hit, Path: "/a"},
				{FirstVisit: true, CreatedAt: hit, Path: "/aa"},
				{FirstVisit: true, CreatedAt: hit, Path: "/aaa"},
				{FirstVisit: true, CreatedAt: hit, Path: "/aaaa"},
			},
			inFilter:   "a",
			inExclude:  []PathID{4, 3},
			wantReturn: "2 false <nil>",
			wantStats: HitLists{
				HitList{Count: 1, Path: "/aa", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
				HitList{Count: 1, Path: "/a", RefScheme: nil, Stats: []HitListStat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx := gctest.DB(t)

			site := MustGetSite(ctx)
			for j := range tt.in {
				if tt.in[j].Site == 0 {
					tt.in[j].Site = site.ID
				}
			}

			gctest.StoreHits(ctx, t, false, tt.in...)

			pathsFilter, err := PathFilter(ctx, tt.inFilter)
			if err != nil {
				t.Fatal(err)
			}

			var stats HitLists
			uniqueDisplay, more, err := stats.List(ctx, rng, pathsFilter, tt.inExclude, 2, GroupHourly)

			have := fmt.Sprintf("%d %t %v", uniqueDisplay, more, err)
			if have != tt.wantReturn {
				t.Errorf("wrong return\nhave: %s\nwant: %s\n", have, tt.wantReturn)
				zdb.Dump(ctx, os.Stdout, "select * from paths")
				zdb.Dump(ctx, os.Stdout, "select * from hit_counts")
				zdb.Dump(ctx, os.Stdout, "select * from hit_stats")
			}

			out := strings.ReplaceAll(", ", ",\n", fmt.Sprintf("%+v", stats))
			want := strings.ReplaceAll(", ", ",\n", fmt.Sprintf("%+v", tt.wantStats))
			if d := ztest.Diff(out, want); d != "" {
				t.Fatal(d)
			}
		})
	}
}

func TestGetTotalCount(t *testing.T) {
	ctx := gctest.DB(t)
	ctx = ztime.WithNow(ctx, ztime.FromString("2020-06-18 12:00:00"))
	rng := ztime.NewRange(ztime.Now(ctx)).To(ztime.Now(ctx))

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/a", FirstVisit: true},
		Hit{Path: "/b", FirstVisit: true},
		Hit{Path: "/a", FirstVisit: false},
		Hit{Path: "ev", FirstVisit: true, Event: true},
		Hit{Path: "ev", FirstVisit: false, Event: true})

	{
		have, err := GetTotalCount(ctx, rng, nil, false)
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"total": 3,
			"total_events": 1,
			"total_utc": 3
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}
}

func TestHitListTotals(t *testing.T) {
	ctx := gctest.DB(t)
	ctx = ztime.WithNow(ctx, ztime.FromString("2020-06-18 12:00:00"))

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/a", FirstVisit: true},
		Hit{Path: "/b", FirstVisit: true},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
		Hit{Path: "/a"},
	)

	t.Run("hourly", func(t *testing.T) {
		rng := ztime.NewRange(ztime.Now(ctx)).To(ztime.Now(ctx))

		want := [][]string{
			{`10`, `{
				"count":  2,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   0
				}]}`},

			{`10`, `{
				"count":  1,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   0
				}]}`},

			{`10`, `{
				"count":  1,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   0
				}]}`},

			{`10`, `{
				"count":  2,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   0
				}]}`},
		}
		for i, filter := range [][]PathID{nil, []PathID{1}, []PathID{2}, []PathID{1, 2}} {
			t.Run("", func(t *testing.T) {
				var hs HitList
				count, err := hs.Totals(ctx, rng, filter, GroupHourly, false)
				if err != nil {
					t.Fatal(err)
				}

				if strconv.Itoa(count) != want[i][0] {
					t.Errorf("count wrong\nhave: %d\nwant: %s", count, want[i][0])
				}
				if d := ztest.Diff(zjson.MustMarshalString(hs), want[i][1], ztest.DiffJSON); d != "" {
					t.Error(d)
				}
			})
		}
	})

	t.Run("daily", func(t *testing.T) {
		rng := ztime.NewRange(ztime.Now(ctx)).To(ztime.Now(ctx))

		want := [][]string{
			{`10`, `{
				"count":  2,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   2
				}]}`},

			{`10`, `{
				"count":  1,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   1
				}]}`},

			{`10`, `{
				"count":  1,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   1
				}]}`},

			{`10`, `{
				"count":  2,
				"path_id":       0,
				"path":          "TOTAL ",
				"event":         false,
				"title":         "",
				"max":           0,
				"stats":[{
					"day":            "2020-06-18",
					"hourly":  [0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],
					"daily":   2
				}]}`},
		}

		for i, filter := range [][]PathID{nil, []PathID{1}, []PathID{2}, []PathID{1, 2}} {
			t.Run("", func(t *testing.T) {
				var hs HitList
				count, err := hs.Totals(ctx, rng, filter, GroupDaily, false)
				if err != nil {
					t.Fatal(err)
				}

				if strconv.Itoa(count) != want[i][0] {
					t.Errorf("count wrong\nhave: %d\nwant: %s", count, want[i][0])
				}
				if d := ztest.Diff(zjson.MustMarshalString(hs), want[i][1], ztest.DiffJSON); d != "" {
					t.Error(d)
				}
			})
		}
	})
}

func TestHitListsPathCount(t *testing.T) {
	ctx := gctest.DB(t)
	ctx = ztime.WithNow(ctx, ztime.FromString("2020-06-18"))

	gctest.StoreHits(ctx, t, false,
		Hit{FirstVisit: true, Path: "/"},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-9 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-9 * 24 * time.Hour)},
		Hit{FirstVisit: false, Path: "/"},

		Hit{FirstVisit: true, Path: "/a"},
		Hit{FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-2 * 24 * time.Hour)},
	)

	{
		var have HitList
		err := have.PathCount(ctx, "/", ztime.Range{})
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"count":  5,
			"event":         false,
			"max":           0,
			"path":          "/",
			"path_id":       0,
			"stats":         null,
			"title":         ""
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}

	{
		var have HitList
		err := have.PathCount(ctx, "/", ztime.NewRange(
			ztime.Now(ctx).Add(-8*24*time.Hour)).
			To(ztime.Now(ctx).Add(-1*24*time.Hour)))
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"count":  2,
			"event":         false,
			"max":           0,
			"path":          "/",
			"path_id":       0,
			"stats":         null,
			"title":         ""
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}
}

func TestHitListSiteTotalUnique(t *testing.T) {
	ctx := gctest.DB(t)
	ctx = ztime.WithNow(ctx, ztime.FromString("2020-06-18"))

	gctest.StoreHits(ctx, t, false,
		Hit{FirstVisit: true, Path: "/"},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-9 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now(ctx).Add(-9 * 24 * time.Hour)},

		Hit{FirstVisit: false, Path: "/"},
		Hit{FirstVisit: true, Path: "/a"},
		Hit{FirstVisit: true, Path: "/a", CreatedAt: ztime.Now(ctx).Add(-2 * 24 * time.Hour)},
	)

	{
		var have HitList
		err := have.SiteTotalUTC(ctx, ztime.Range{})
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"count":  7,
			"event":         false,
			"max":           0,
			"path":          "",
			"path_id":       0,
			"stats":         null,
			"title":         ""
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}

	{
		var have HitList
		err := have.SiteTotalUTC(ctx, ztime.NewRange(
			ztime.Now(ctx).Add(-8*24*time.Hour)).
			To(ztime.Now(ctx).Add(-1*24*time.Hour)))
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"count":  3,
			"event":         false,
			"max":           0,
			"path":          "",
			"path_id":       0,
			"stats":         null,
			"title":         ""
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}
}

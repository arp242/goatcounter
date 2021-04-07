// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
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
		inExclude  []int64
		wantReturn string
		wantStats  HitLists
	}{
		{
			in: []Hit{
				{CreatedAt: hit, Path: "/asd"},
				{CreatedAt: hit.Add(40 * time.Hour), Path: "/asd/"},
				{CreatedAt: hit.Add(100 * time.Hour), Path: "/zxc"},
			},
			wantReturn: "3 0 false <nil>",
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
				{CreatedAt: hit, Path: "/asd"},
				{CreatedAt: hit, Path: "/zxc"},
			},
			inFilter:   "x",
			wantReturn: "1 0 false <nil>",
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
				{CreatedAt: hit, Path: "/a"},
				{CreatedAt: hit, Path: "/aa"},
				{CreatedAt: hit, Path: "/aaa"},
				{CreatedAt: hit, Path: "/aaaa"},
			},
			inFilter:   "a",
			wantReturn: "2 0 true <nil>",
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
				{CreatedAt: hit, Path: "/a"},
				{CreatedAt: hit, Path: "/aa"},
				{CreatedAt: hit, Path: "/aaa"},
				{CreatedAt: hit, Path: "/aaaa"},
			},
			inFilter:   "a",
			inExclude:  []int64{4, 3},
			wantReturn: "2 0 false <nil>",
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
			user := MustGetUser(ctx)
			for j := range tt.in {
				if tt.in[j].Site == 0 {
					tt.in[j].Site = site.ID
				}
			}

			user.Settings.Widgets = Widgets{
				{"on": true, "name": "pages", "s": map[string]interface{}{"limit_pages": float64(2), "limit_refs": float64(10)}},
			}

			gctest.StoreHits(ctx, t, false, tt.in...)

			pathsFilter, err := PathFilter(ctx, tt.inFilter, true)
			if err != nil {
				t.Fatal(err)
			}

			var stats HitLists
			totalDisplay, uniqueDisplay, more, err := stats.List(ctx, rng, pathsFilter, tt.inExclude, false)

			got := fmt.Sprintf("%d %d %t %v", totalDisplay, uniqueDisplay, more, err)
			if got != tt.wantReturn {
				t.Errorf("wrong return\nout:  %s\nwant: %s\n", got, tt.wantReturn)
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

func TestGetMax(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:00:00")
	ctx := gctest.DB(t)

	rng := ztime.NewRange(ztime.Now()).To(ztime.Now())

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/a"},
		Hit{Path: "/b"},
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
		want := []int{11, 11, 10, 11}
		for i, filter := range [][]int64{nil, []int64{1}, []int64{2}, []int64{1, 2}} {
			got, err := GetMax(ctx, rng, filter, false)
			if err != nil {
				t.Fatal(err)
			}
			w := want[i]
			if got != w {
				t.Errorf("got %d; want %d (filter=%v)", got, w, filter)
			}
		}
	})

	t.Run("daily", func(t *testing.T) {
		want := []int{11, 11, 10, 11}
		for i, filter := range [][]int64{nil, []int64{1}, []int64{2}, []int64{1, 2}} {
			got, err := GetMax(ctx, rng, filter, true)
			if err != nil {
				t.Fatal(err)
			}
			w := want[i]
			if got != w {
				t.Errorf("got %d; want %d (filter=%v)", got, w, filter)
			}
		}
	})
}

func TestGetTotalCount(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:00:00")
	ctx := gctest.DB(t)

	rng := ztime.NewRange(ztime.Now()).To(ztime.Now())

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/a", FirstVisit: true},
		Hit{Path: "/b", FirstVisit: true},
		Hit{Path: "/a", FirstVisit: false},
		Hit{Path: "ev", FirstVisit: true, Event: true},
		Hit{Path: "ev", FirstVisit: false, Event: true})

	{
		tt, err := GetTotalCount(ctx, rng, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "{5 3 3 2 1}"
		got := fmt.Sprintf("%v", tt)
		if want != got {
			t.Errorf("\nwant: %s\ngot:  %s", want, got)
		}
	}
}

func TestHitListTotals(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:00:00")
	ctx := gctest.DB(t)

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
		rng := ztime.NewRange(ztime.Now()).To(ztime.Now())

		want := []string{
			`12 {"Count":12,"CountUnique":2,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,12,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],"Daily":0,"DailyUnique":0}]}`,

			`11 {"Count":11,"CountUnique":1,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,11,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"Daily":0,"DailyUnique":0}]}`,

			`10 {"Count":1,"CountUnique":1,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"Daily":0,"DailyUnique":0}]}`,

			`12 {"Count":12,"CountUnique":2,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,12,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],"Daily":0,"DailyUnique":0}]}`,
		}
		for i, filter := range [][]int64{nil, []int64{1}, []int64{2}, []int64{1, 2}} {
			var hs HitList
			count, err := hs.Totals(ctx, rng, filter, false)
			if err != nil {
				t.Fatal(err)
			}

			got := fmt.Sprintf("%d %s", count, zjson.MustMarshal(hs))
			w := want[i]
			if got != w {
				t.Errorf("\nwant: %s\ngot:  %v\nfilter: %v", w, got, filter)
			}
		}
	})

	t.Run("daily", func(t *testing.T) {
		rng := ztime.NewRange(ztime.Now()).To(ztime.Now())

		want := []string{
			`12 {"Count":12,"CountUnique":2,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,12,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],"Daily":12,"DailyUnique":2}]}`,

			`11 {"Count":11,"CountUnique":1,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,11,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"Daily":11,"DailyUnique":1}]}`,

			`10 {"Count":1,"CountUnique":1,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"Daily":1,"DailyUnique":1}]}`,

			`12 {"Count":12,"CountUnique":2,"PathID":0,"Path":"TOTAL ","Event":false,"Title":"","RefScheme":null,"Max":0,"Stats":[` +
				`{"Day":"2020-06-18","Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,12,0,0,0,0,0,0,0,0,0,0,0],"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0],"Daily":12,"DailyUnique":2}]}`,
		}

		for i, filter := range [][]int64{nil, []int64{1}, []int64{2}, []int64{1, 2}} {
			var hs HitList
			count, err := hs.Totals(ctx, rng, filter, true)
			if err != nil {
				t.Fatal(err)
			}

			got := fmt.Sprintf("%d %s", count, zjson.MustMarshal(hs))
			w := want[i]
			if got != w {
				t.Errorf("\nwant: %s\ngot:  %v\nfilter: %v", w, got, filter)
			}
		}
	})
}

func TestHitListsPathCountUnique(t *testing.T) {
	ztime.SetNow(t, "2020-06-18")
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{FirstVisit: true, Path: "/"},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-9 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-9 * 24 * time.Hour)},

		Hit{FirstVisit: false, Path: "/"},
		Hit{FirstVisit: true, Path: "/a"},
		Hit{FirstVisit: true, Path: "/a", CreatedAt: ztime.Now().Add(-2 * 24 * time.Hour)},
	)

	{
		var hl HitList
		err := hl.PathCountUnique(ctx, "/", ztime.Range{})
		if err != nil {
			t.Fatal(err)
		}
		want := `{0 5 0 / false  <nil> 0 []}`
		have := fmt.Sprintf("%v", hl)
		if have != want {
			t.Errorf("\nhave: %#v\nwant: %#v", have, want)
		}
	}

	{
		var hl HitList
		err := hl.PathCountUnique(ctx, "/", ztime.NewRange(
			ztime.Now().Add(-8*24*time.Hour)).
			To(ztime.Now().Add(-1*24*time.Hour)))
		if err != nil {
			t.Fatal(err)
		}
		want := `{0 2 0 / false  <nil> 0 []}`
		have := fmt.Sprintf("%v", hl)
		if have != want {
			t.Errorf("\nhave: %#v\nwant: %#v", have, want)
		}
	}
}

func TestHitListSiteTotalUnique(t *testing.T) {
	ztime.SetNow(t, "2020-06-18")
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{FirstVisit: true, Path: "/"},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-2 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-9 * 24 * time.Hour)},
		Hit{FirstVisit: true, Path: "/", CreatedAt: ztime.Now().Add(-9 * 24 * time.Hour)},

		Hit{FirstVisit: false, Path: "/"},
		Hit{FirstVisit: true, Path: "/a"},
		Hit{FirstVisit: true, Path: "/a", CreatedAt: ztime.Now().Add(-2 * 24 * time.Hour)},
	)

	{
		var hl HitList
		err := hl.SiteTotalUTC(ctx, ztime.Range{})
		if err != nil {
			t.Fatal(err)
		}
		want := `{8 7 0  false  <nil> 0 []}`
		have := fmt.Sprintf("%v", hl)
		if have != want {
			t.Errorf("\nhave: %#v\nwant: %#v", have, want)
		}
	}

	{
		var hl HitList
		err := hl.SiteTotalUTC(ctx, ztime.NewRange(
			ztime.Now().Add(-8*24*time.Hour)).
			To(ztime.Now().Add(-1*24*time.Hour)))
		if err != nil {
			t.Fatal(err)
		}
		want := `{3 3 0  false  <nil> 0 []}`
		have := fmt.Sprintf("%v", hl)
		if have != want {
			t.Errorf("\nhave: %#v\nwant: %#v", have, want)
		}
	}
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"fmt"
	"testing"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zstd/zjson"
)

func TestGetMax(t *testing.T) {
	defer gctest.SwapNow(t, "2020-06-18 12:00:00")()
	ctx, clean := gctest.DB(t)
	defer clean()

	start := Now()
	end := Now()

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
			got, err := GetMax(ctx, start, end, filter, false)
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
			got, err := GetMax(ctx, start, end, filter, true)
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
	defer gctest.SwapNow(t, "2020-06-18 12:00:00")()
	ctx, clean := gctest.DB(t)
	defer clean()

	start := Now()
	end := Now()

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/a", FirstVisit: true},
		Hit{Path: "/b", FirstVisit: true},
		Hit{Path: "/a", FirstVisit: false},
		Hit{Path: "ev", FirstVisit: true, Event: true},
		Hit{Path: "ev", FirstVisit: false, Event: true})

	{
		tt, err := GetTotalCount(ctx, start, end, nil)
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

func TestHitStatTotals(t *testing.T) {
	defer gctest.SwapNow(t, "2020-06-18 12:00:00")()
	ctx, clean := gctest.DB(t)
	defer clean()

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
		start := Now()
		end := Now()

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
			var hs HitStat
			count, err := hs.Totals(ctx, start, end, filter, false)
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
		start := Now()
		end := Now()

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
			var hs HitStat
			count, err := hs.Totals(ctx, start, end, filter, true)
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

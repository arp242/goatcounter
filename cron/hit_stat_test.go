// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztime"
)

func TestHitStats(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/asd", Title: "aSd", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Path: "/asd/"}, // Trailing / should be sanitized and treated identical as /asd
		{Site: site.ID, CreatedAt: now, Path: "/zxc"},
	}...)

	check := func(wantT, want0, want1 string) {
		var stats goatcounter.HitLists
		display, displayUnique, more, err := stats.List(ctx,
			ztime.NewRange(now.Add(-1*time.Hour)).To(now.Add(1*time.Hour)),
			nil, nil, 10, false)
		if err != nil {
			t.Fatal(err)
		}

		gotT := fmt.Sprintf("%d %d %t", display, displayUnique, more)
		if wantT != gotT {
			t.Fatalf("wrong totals\ngot:  %s\nwant: %s", gotT, wantT)
		}
		if len(stats) != 2 {
			t.Fatalf("len(stats) is not 2: %d", len(stats))
		}

		got0 := string(zjson.MustMarshal(stats[0]))
		if got0 != want0 {
			t.Errorf("first wrong\ngot:  %s\nwant: %s", got0, want0)
		}

		got1 := string(zjson.MustMarshal(stats[1]))
		if got1 != want1 {
			t.Errorf("second wrong\ngot:  %s\nwant: %s", got1, want1)
		}
	}

	check("3 1 false",
		`{"Count":2,"CountUnique":1,"PathID":1,"Path":"/asd","Event":false,"Title":"aSd","RefScheme":null,"Max":2,`+
			`"Stats":[{"Day":"2019-08-31",`+
			`"Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0],`+
			`"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],"Daily":2,"DailyUnique":1}]}`,
		`{"Count":1,"CountUnique":0,"PathID":2,"Path":"/zxc","Event":false,"Title":"","RefScheme":null,"Max":1,`+
			`"Stats":[{"Day":"2019-08-31",`+
			`"Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],`+
			`"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"Daily":1,"DailyUnique":0}]}`,
	)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now.Add(2 * time.Hour), Path: "/asd", Title: "aSd", FirstVisit: true},
		{Site: site.ID, CreatedAt: now.Add(2 * time.Hour), Path: "/asd", Title: "aSd"},
	}...)

	check("5 2 false",
		`{"Count":4,"CountUnique":2,"PathID":1,"Path":"/asd","Event":false,"Title":"aSd","RefScheme":null,"Max":2,`+
			`"Stats":[{"Day":"2019-08-31",`+
			`"Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,2,0,0,0,0,0,0,0],`+
			`"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,1,0,0,0,0,0,0,0],"Daily":4,"DailyUnique":2}]}`,
		`{"Count":1,"CountUnique":0,"PathID":2,"Path":"/zxc","Event":false,"Title":"","RefScheme":null,"Max":1,`+
			`"Stats":[{"Day":"2019-08-31",`+
			`"Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],`+
			`"HourlyUnique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"Daily":1,"DailyUnique":0}]}`,
	)
}

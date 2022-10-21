// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zjson"
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
		`{"count":2,"count_unique":1,"path_id":1,"path":"/asd","event":false,"title":"aSd","max":2,`+
			`"stats":[{"day":"2019-08-31",`+
			`"hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0],`+
			`"hourly_unique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],"daily":2,"daily_unique":1}]}`,
		`{"count":1,"count_unique":0,"path_id":2,"path":"/zxc","event":false,"title":"","max":1,`+
			`"stats":[{"day":"2019-08-31",`+
			`"hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],`+
			`"hourly_unique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"daily":1,"daily_unique":0}]}`,
	)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now.Add(2 * time.Hour), Path: "/asd", Title: "aSd", FirstVisit: true},
		{Site: site.ID, CreatedAt: now.Add(2 * time.Hour), Path: "/asd", Title: "aSd"},
	}...)

	check("5 2 false",
		`{"count":4,"count_unique":2,"path_id":1,"path":"/asd","event":false,"title":"aSd","max":2,`+
			`"stats":[{"day":"2019-08-31",`+
			`"hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,2,0,0,0,0,0,0,0],`+
			`"hourly_unique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,1,0,0,0,0,0,0,0],"daily":4,"daily_unique":2}]}`,
		`{"count":1,"count_unique":0,"path_id":2,"path":"/zxc","event":false,"title":"","max":1,`+
			`"stats":[{"day":"2019-08-31",`+
			`"hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],`+
			`"hourly_unique":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"daily":1,"daily_unique":0}]}`,
	)
}

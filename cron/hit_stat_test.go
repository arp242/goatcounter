// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron_test

import (
	"testing"
	"time"

	"zgo.at/goatcounter"
	. "zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
	"zgo.at/utils/jsonutil"
)

func TestHitStats(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	goatcounter.Memstore.Append([]goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/asd", Title: "aSd"},
		{Site: site.ID, CreatedAt: now, Path: "/asd/"}, // Trailing / should be sanitized and treated identical as /asd
		{Site: site.ID, CreatedAt: now, Path: "/zxc"},
	}...)
	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		panic(err)
	}

	err = UpdateStats(ctx, site.ID, hits)
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.HitStats
	total, display, more, err := stats.List(ctx, now, now, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if total != 3 || display != 3 || more {
		t.Fatalf("wrong return\nwant: 3, 3, false\ngot:  %v, %v, %v", total, display, more)
	}
	if len(stats) != 2 {
		t.Fatalf("len(stats) is not 2: %d", len(stats))
	}

	want0 := `{"Count":2,"Max":10,"Path":"/asd","Event":false,"Title":"aSd","RefScheme":null,"Stats":[{"Day":"2019-08-31","Days":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0],"Total":2}]}`
	got0 := string(jsonutil.MustMarshal(stats[0]))
	if got0 != want0 {
		t.Errorf("first wrong\ngot:  %s\nwant: %s", got0, want0)
	}

	want1 := `{"Count":1,"Max":10,"Path":"/zxc","Event":false,"Title":"","RefScheme":null,"Stats":[{"Day":"2019-08-31","Days":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0],"Total":1}]}`
	got1 := string(jsonutil.MustMarshal(stats[1]))
	if got1 != want1 {
		t.Errorf("second wrong\ngot:  %s\nwant: %s", got1, want1)
	}
}

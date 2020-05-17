// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
	. "zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
)

func TestDataRetention(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	site := goatcounter.Site{Code: "bbbb", Plan: goatcounter.PlanPersonal,
		Settings: goatcounter.SiteSettings{DataRetention: 30}}
	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx = goatcounter.WithSite(ctx, &site)

	now := time.Now().UTC()
	past := now.Add(-40 * 24 * time.Hour)

	gctest.StoreHits(ctx, t, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/a"},
		{Site: site.ID, CreatedAt: now, Path: "/a"},
		{Site: site.ID, CreatedAt: past, Path: "/a"},
		{Site: site.ID, CreatedAt: past, Path: "/a"},
	}...)

	err = DataRetention(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var hits goatcounter.Hits
	_, err = hits.List(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(hits) != 2 {
		t.Errorf("len(hits) is %d\n%v", len(hits), hits)
	}

	var stats goatcounter.HitStats
	total, totalUnique, display, displayUnique, more, err := stats.List(ctx, past.Add(-1*24*time.Hour), now, "", nil)
	_, _ = totalUnique, displayUnique // TODO
	if err != nil {
		t.Fatal(err)
	}

	out := fmt.Sprintf("%d %d %t %v", total, display, more, err)
	want := `2 2 false <nil>`
	if out != want {
		t.Errorf("\ngot:  %s\nwant: %s", out, want)
	}
}

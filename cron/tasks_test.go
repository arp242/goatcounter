// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/ztime"
)

func TestDataRetention(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.Site{Code: "bbbb", Settings: goatcounter.SiteSettings{DataRetention: 31}}
	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx = goatcounter.WithSite(ctx, &site)

	now := time.Now().UTC()
	past := now.Add(-40 * 24 * time.Hour)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/a", FirstVisit: zbool.Bool(true)},
		{Site: site.ID, CreatedAt: now, Path: "/a", FirstVisit: zbool.Bool(false)},
		{Site: site.ID, CreatedAt: past, Path: "/a", FirstVisit: zbool.Bool(true)},
		{Site: site.ID, CreatedAt: past, Path: "/a", FirstVisit: zbool.Bool(false)},
	}...)

	err = cron.TaskDataRetention()
	if err != nil {
		t.Fatal(err)
	}
	cron.WaitDataRetention()

	var hits goatcounter.Hits
	err = hits.TestList(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("len(hits) is %d\n%v", len(hits), hits)
	}

	var stats goatcounter.HitLists
	display, more, err := stats.List(ctx,
		ztime.NewRange(past.Add(-1*24*time.Hour)).To(now),
		nil, nil, 10, false)
	if err != nil {
		t.Fatal(err)
	}

	out := fmt.Sprintf("%d %t %v", display, more, err)
	want := `1 false <nil>`
	if out != want {
		t.Errorf("\ngot:  %s\nwant: %s", out, want)
	}
}

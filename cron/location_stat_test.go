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
	"zgo.at/zstd/ztime"
)

func TestLocationStats(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
	}...)

	var stats goatcounter.HitStats
	err := stats.ListLocations(ctx, ztime.NewRange(now).To(now), nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{ET Ethiopia 1 1 <nil>} {ID Indonesia 2 0 <nil>}]}`
	out := fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "NZ"},
	}...)

	stats = goatcounter.HitStats{}
	err = stats.ListLocations(ctx, ztime.NewRange(now).To(now), nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{ET Ethiopia 5 3 <nil>} {ID Indonesia 4 0 <nil>} {NZ New Zealand 1 0 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at eupl12.zgo.at

package cron

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
)

func TestLocationStats(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	err := updateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.BrowserStats
	total, err := stats.ListLocations(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want := `3 -> [{Indonesia false 2} {Ethiopia false 1}]`
	out := fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	err = updateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "NZ"},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = goatcounter.BrowserStats{}
	total, err = stats.ListLocations(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `10 -> [{Ethiopia false 5} {Indonesia false 4} {New Zealand false 1}]`
	out = fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

}

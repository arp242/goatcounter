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

func TestLocationStats(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	err := UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET", StartedSession: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.Stats
	total, err := stats.ListLocations(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want := `3 -> [{Indonesia 2 0} {Ethiopia 1 1}]`
	out := fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	err = UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET", StartedSession: true},
		{Site: site.ID, CreatedAt: now, Location: "ET", StartedSession: true},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "NZ"},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = goatcounter.Stats{}
	total, err = stats.ListLocations(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `10 -> [{Ethiopia 5 3} {Indonesia 4 0} {New Zealand 1 0}]`
	out = fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

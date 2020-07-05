// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

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
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.Stats
	err = stats.ListLocations(ctx, now, now, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{Ethiopia 1 1 <nil>} {Indonesia 2 0 <nil>}]}`
	out := fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	err = UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "NZ"},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = goatcounter.Stats{}
	err = stats.ListLocations(ctx, now, now, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{Ethiopia 5 3 <nil>} {Indonesia 4 0 <nil>} {New Zealand 1 0 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

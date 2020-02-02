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

func TestBrowserStats(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	err := UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/68.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Chrome/77.0.123.666"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.Stats
	total, err := stats.ListBrowsers(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want := `4 -> [{Firefox 3} {Chrome 1}]`
	out := fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	err = UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/70.0"},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = goatcounter.Stats{}
	total, err = stats.ListBrowsers(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `7 -> [{Firefox 6} {Chrome 1}]`
	out = fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// List just Firefox.
	stats = goatcounter.Stats{}
	total, err = stats.ListBrowser(ctx, "Firefox", now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `6 -> [{Firefox 69.0 4} {Firefox 68.0 1} {Firefox 70.0 1}]`
	out = fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

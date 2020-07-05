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

func TestBrowserStats(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	err := UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/68.0", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Browser: "Chrome/77.0.123.666"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.Stats
	err = stats.ListBrowsers(ctx, now, now, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{Firefox 3 1 <nil>} {Chrome 1 0 <nil>}]}`
	out := fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	err = UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/70.0"},
		{Site: site.ID, CreatedAt: now, Browser: "Firefox/70.0"},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = goatcounter.Stats{}
	err = stats.ListBrowsers(ctx, now, now, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{Firefox 7 2 <nil>} {Chrome 1 0 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// List just Firefox.
	stats = goatcounter.Stats{}
	err = stats.ListBrowser(ctx, "Firefox", now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{Firefox 68 1 1 <nil>} {Firefox 69 4 1 <nil>} {Firefox 70 2 0 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

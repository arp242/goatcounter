// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
)

func TestBrowserStats(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/68.0", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Chrome/77.0.123.666"},
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/69.0"},
	}...)

	var stats goatcounter.HitStats
	err := stats.ListBrowsers(ctx, now, now, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{ Firefox 3 1 <nil>} { Chrome 1 0 <nil>}]}`
	out := fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/69.0", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/69.0"},
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/70.0"},
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Firefox/70.0"},
	}...)

	stats = goatcounter.HitStats{}
	err = stats.ListBrowsers(ctx, now, now, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{ Firefox 7 2 <nil>} { Chrome 1 0 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// List just Firefox.
	stats = goatcounter.HitStats{}
	err = stats.ListBrowser(ctx, "Firefox", now, now, nil)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{ Firefox 68 1 1 <nil>} { Firefox 69 4 1 <nil>} { Firefox 70 2 0 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

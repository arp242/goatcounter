// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
)

func TestBrowserStats(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	// Insert some browsers.
	browsers := []goatcounter.Hit{
		{Browser: "Firefox/68.0", CreatedAt: now},
		{Browser: "Chrome/77.0.123.666", CreatedAt: now},
		{Browser: "Firefox/69.0", CreatedAt: now},
	}
	for _, b := range browsers {
		b.Site = site.ID
		err := b.Insert(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := updateStats(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.BrowserStats
	total, err := stats.List(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want := `3 -> [{Firefox 2} {Chrome 1}]`
	out := fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	stats = goatcounter.BrowserStats{}
	total, err = stats.ListBrowser(ctx, "Firefox", now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `2 -> [{68.0 1} {69.0 1}]`
	out = fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

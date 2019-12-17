// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

/*
func TestBrowserStats(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	goatcounter.Memstore.Append([]goatcounter.Hit{
		{Site: site.ID, Browser: "Firefox/68.0", CreatedAt: now},
		{Site: site.ID, Browser: "Chrome/77.0.123.666", CreatedAt: now},
		{Site: site.ID, Browser: "Firefox/69.0", CreatedAt: now},
	}...)
	_, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = updateStats(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.BrowserStats
	total, totalMobile, err := stats.List(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want := `3 -> 0 -> [{Firefox false 2} {Chrome false 1}]`
	out := fmt.Sprintf("%d -> %d -> %v", total, totalMobile, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	stats = goatcounter.BrowserStats{}
	total, err = stats.ListBrowser(ctx, "Firefox", now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `2 -> [{Firefox 68.0 false 1} {Firefox 69.0 false 1}]`
	out = fmt.Sprintf("%d -> %v", total, stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}
*/

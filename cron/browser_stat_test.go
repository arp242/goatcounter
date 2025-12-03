package cron_test

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/ztime"
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
	err := stats.ListBrowsers(ctx, ztime.NewRange(now).To(now), goatcounter.PathFilter{}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{ Firefox 1 <nil>}]}`
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
		{Site: site.ID, CreatedAt: now, UserAgentHeader: "Chrome/77.0.123.666", FirstVisit: true},
	}...)

	stats = goatcounter.HitStats{}
	err = stats.ListBrowsers(ctx, ztime.NewRange(now).To(now), goatcounter.PathFilter{}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{ Firefox 2 <nil>} { Chrome 1 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// List just Firefox.
	stats = goatcounter.HitStats{}
	err = stats.ListBrowser(ctx, "Firefox", ztime.NewRange(now).To(now), goatcounter.PathFilter{}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{ Firefox 68 1 <nil>} { Firefox 69 1 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

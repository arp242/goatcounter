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

	want := `{false [{ET Ethiopia 1 <nil>}]}`
	out := fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}

	// Update existing.
	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Location: "ID"},
		{Site: site.ID, CreatedAt: now, Location: "ID", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET", FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Location: "ET"},
		{Site: site.ID, CreatedAt: now, Location: "NZ", FirstVisit: true},
	}...)

	stats = goatcounter.HitStats{}
	err = stats.ListLocations(ctx, ztime.NewRange(now).To(now), nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{ET Ethiopia 3 <nil>} {ID Indonesia 1 <nil>} {NZ New Zealand 1 <nil>}]}`
	out = fmt.Sprintf("%v", stats)
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

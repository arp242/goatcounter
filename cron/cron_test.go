package cron

import (
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
)

func TestDataRetention(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
	past := now.Add(-15 * 24 * time.Hour)

	goatcounter.Memstore.Append([]goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/a"},
		{Site: site.ID, CreatedAt: now, Path: "/a"},
		{Site: site.ID, CreatedAt: past, Path: "/a"},
		{Site: site.ID, CreatedAt: past, Path: "/a"},
	}...)
	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		panic(err)
	}
	err = updateStats(ctx, site.ID, hits)
	if err != nil {
		t.Fatal(err)
	}

	err = dataRetention(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.HitStats
	total, display, more, err := stats.List(ctx, now, now, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	out := fmt.Sprintf("%d %d %t %v %v", total, display, more, err, stats)
	want := `2 2 false <nil> [{2 10 /a  <nil> [{2019-08-31 [0 0 0 0 0 0 0 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0 0 0]}]}]`
	if out != want {
		t.Errorf("\ngot:  %s\nwant: %s", out, want)
	}
}

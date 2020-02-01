package cron

import (
	"context"
	"fmt"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/zhttp/ctxkey"
)

func TestDataRetention(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	site := goatcounter.Site{Code: "bbbb", Name: "bbbb", Plan: goatcounter.PlanPersonal,
		Settings: goatcounter.SiteSettings{DataRetention: 30}}
	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx = context.WithValue(ctx, ctxkey.Site, &site)

	now := time.Now().UTC()
	past := now.Add(-40 * 24 * time.Hour)

	StoreHits(ctx, t, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Path: "/a"},
		{Site: site.ID, CreatedAt: now, Path: "/a"},
		{Site: site.ID, CreatedAt: past, Path: "/a"},
		{Site: site.ID, CreatedAt: past, Path: "/a"},
	}...)

	err = dataRetention(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var hits goatcounter.Hits
	err = hits.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(hits) != 2 {
		t.Errorf("len(hits) is %d\n%v", len(hits), hits)
	}

	var stats goatcounter.HitStats
	total, display, more, err := stats.List(ctx, past.Add(-1*24*time.Hour), now, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	out := fmt.Sprintf("%d %d %t %v", total, display, more, err)
	want := `2 2 false <nil>`
	if out != want {
		t.Errorf("\ngot:  %s\nwant: %s", out, want)
	}
}

func StoreHits(ctx context.Context, t *testing.T, hits ...goatcounter.Hit) []goatcounter.Hit {
	t.Helper()

	goatcounter.Memstore.Append(hits...)
	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		t.Fatal(err)
	}

	sites := make(map[int64]struct{})
	for _, h := range hits {
		sites[h.Site] = struct{}{}
	}

	for s := range sites {
		err = updateStats(ctx, s, hits)
		if err != nil {
			t.Fatal(err)
		}
	}

	return hits
}

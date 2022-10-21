// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestSizeStats(t *testing.T) {
	ctx := gctest.DB(t)

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}, FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{1024, 768, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{}},
		{Site: site.ID, CreatedAt: now, Size: nil},
	}...)

	var stats goatcounter.HitStats
	err := stats.ListSizes(ctx, ztime.NewRange(now).To(now), nil)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{phone Phones 0 0 <nil>}
{largephone Large phones, small tablets 1 0 <nil>}
{tablet Tablets and small laptops 0 0 <nil>}
{desktop Computer monitors 2 1 <nil>}
{desktophd Computer monitors larger than HD 0 0 <nil>}
{unknown (unknown) 2 0 <nil>}]}`
	out := strings.ReplaceAll(fmt.Sprintf("%v", stats), "} ", "}\n")
	if want != out {
		t.Error(ztest.Diff(out, want, ztest.DiffVerbose))
	}

	// Update existing.
	gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{1024, 768, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}, FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Size: []float64{1024, 768, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{380, 600, 1}},
		{Site: site.ID, CreatedAt: now, Size: nil, FirstVisit: true},
	}...)

	stats = goatcounter.HitStats{}
	err = stats.ListSizes(ctx, ztime.NewRange(now).To(now), nil)
	if err != nil {
		t.Fatal(err)
	}

	want = `{false [{phone Phones 1 0 <nil>}
{largephone Large phones, small tablets 3 0 <nil>}
{tablet Tablets and small laptops 0 0 <nil>}
{desktop Computer monitors 4 2 <nil>}
{desktophd Computer monitors larger than HD 0 0 <nil>}
{unknown (unknown) 3 1 <nil>}]}`
	out = strings.ReplaceAll(fmt.Sprintf("%v", stats), "} ", "}\n")
	if want != out {
		t.Error(ztest.Diff(out, want, ztest.DiffVerbose))
	}
}

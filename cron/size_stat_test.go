// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter"
	. "zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
)

func TestSizeStats(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	err := UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}, FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{1024, 768, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{}},
		{Site: site.ID, CreatedAt: now, Size: nil},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.Stats
	total, err := stats.ListSizes(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want := `5 -> [{Phones 0 0}
{Large phones, small tablets 1 0}
{Tablets and small laptops 0 0}
{Computer monitors 2 1}
{Computer monitors larger than HD 0 0}
{(unknown) 2 0}]`
	out := strings.ReplaceAll(fmt.Sprintf("%d -> %v", total, stats), "} ", "}\n")
	if want != out {
		t.Errorf("\nwant:\n%s\nout:\n%s", want, out)
	}

	// Update existing.
	err = UpdateStats(ctx, site.ID, []goatcounter.Hit{
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{1024, 768, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{1920, 1080, 1}, FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Size: []float64{1024, 768, 1}},
		{Site: site.ID, CreatedAt: now, Size: []float64{380, 600, 1}},
		{Site: site.ID, CreatedAt: now, Size: nil, FirstVisit: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = goatcounter.Stats{}
	total, err = stats.ListSizes(ctx, now, now)
	if err != nil {
		t.Fatal(err)
	}

	want = `11 -> [{Phones 1 0}
{Large phones, small tablets 3 0}
{Tablets and small laptops 0 0}
{Computer monitors 4 2}
{Computer monitors larger than HD 0 0}
{(unknown) 3 1}]`
	out = strings.ReplaceAll(fmt.Sprintf("%d -> %v", total, stats), "} ", "}\n")
	if want != out {
		t.Errorf("\nwant: %s\nout:  %s", want, out)
	}
}

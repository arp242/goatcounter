// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron_test

import (
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zjson"
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
		{Site: site.ID, CreatedAt: now, Size: []float64{1708.6443250402808, 872.214019395411, 1}, FirstVisit: true},
		{Site: site.ID, CreatedAt: now, Size: []float64{}},
		{Site: site.ID, CreatedAt: now, Size: nil},
	}...)

	var have goatcounter.HitStats
	err := have.ListSizes(ctx, ztime.NewRange(now).To(now), nil)
	if err != nil {
		t.Fatal(err)
	}

	want := `{
		"more": false,
		"stats": [
			{"count": 0, "id": "phone", "name": ""},
			{"count": 0, "id": "largephone", "name": "" },
			{"count": 0, "id": "tablet", "name": ""},
			{"count": 2, "id": "desktop", "name": ""},
			{"count": 0, "id": "desktophd", "name": ""},
			{"count": 0, "id": "unknown", "name": ""}
		]
	}`

	if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
		t.Error(d)
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

	have = goatcounter.HitStats{}
	err = have.ListSizes(ctx, ztime.NewRange(now).To(now), nil)
	if err != nil {
		t.Fatal(err)
	}

	want = `{
		"more": false,
		"stats": [
			{"count": 0, "id": "phone", "name": ""},
			{"count": 0, "id": "largephone", "name": "" },
			{"count": 0, "id": "tablet", "name": ""},
			{"count": 3, "id": "desktop", "name": ""},
			{"count": 0, "id": "desktophd", "name": ""},
			{"count": 1, "id": "unknown", "name": ""}
		]
	}`
	if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
		t.Error(d)
	}
}

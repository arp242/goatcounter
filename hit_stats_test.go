// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"strings"
	"testing"
	"time"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestHitStats(t *testing.T) {
	ctx := gctest.DB(t)

	s := MustGetSite(ctx)
	s.Settings.CollectRegions = Strings{}
	err := s.Update(ctx)
	if err != nil {
		t.Fatal(err)
	}

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/x", Location: "NL-NB", Size: []float64{1920, 1080, 1}, UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:81.0) Gecko/20100101 Firefox/81.0", FirstVisit: true},
		Hit{Path: "/x", Location: "NL-NB", Size: []float64{1920, 1080, 1}, UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:81.0) Gecko/20100101 Firefox/81.0"},
		Hit{Path: "/y", Location: "ID-BA", Size: []float64{800, 600, 2}, UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; Ubuntu; rv:79.0) Gecko/20100101 Firefox/79.0", FirstVisit: true},
	)

	rng := ztime.NewRange(ztime.Now()).To(ztime.Now())

	cmp := func(t *testing.T, want string, stats ...HitStats) {
		t.Helper()

		var got string
		for _, s := range stats {
			got += string(zjson.MustMarshalIndent(s, "\t\t\t", "\t"))
		}
		if d := ztest.Diff(got, want); d != "" {
			t.Error(d)
		}
	}

	for _, filter := range [][]int64{nil} {
		// Browsers
		{
			var list HitStats
			err := list.ListBrowsers(ctx, rng, filter, 5, 0)
			if err != nil {
				t.Fatal(err)
			}
			var get HitStats
			err = get.ListBrowser(ctx, "Firefox", rng, filter, 10, 0)
			if err != nil {
				t.Fatal(err)
			}
			cmp(t, `{
				"more": false,
				"stats": [
					{
						"name": "Firefox",
						"count": 2
					}
				]
			}{
				"more": false,
				"stats": [
					{
						"name": "Firefox 79",
						"count": 1
					},
					{
						"name": "Firefox 81",
						"count": 1
					}
				]
			}`, list, get)
		}

		// Systems
		{
			var list HitStats
			err := list.ListSystems(ctx, rng, filter, 5, 0)
			if err != nil {
				t.Fatal(err)
			}
			var get HitStats
			err = get.ListSystem(ctx, "Linux", rng, filter, 10, 0)
			if err != nil {
				t.Fatal(err)
			}
			cmp(t, `{
				"more": false,
				"stats": [
					{
						"name": "Linux",
						"count": 2
					}
				]
			}{
				"more": false,
				"stats": [
					{
						"name": "Linux",
						"count": 1
					},
					{
						"name": "Linux Ubuntu",
						"count": 1
					}
				]
			}`, list, get)
		}

		// Sizes
		{
			var list HitStats
			err := list.ListSizes(ctx, rng, filter)
			if err != nil {
				t.Fatal(err)
			}
			var get HitStats
			err = get.ListSize(ctx, "desktop", rng, filter, 10, 0)
			if err != nil {
				t.Fatal(err)
			}
			cmp(t, strings.ReplaceAll(`{
				"more": false,
				"stats": [
					{
						"id": "phone",
						"name": "Phones",
						"count": 0
					},
					{
						"id": "largephone",
						"name": "Large phones, small tablets",
						"count": 1
					},
					{
						"id": "tablet",
						"name": "Tablets and small laptops",
						"count": 0
					},
					{
						"id": "desktop",
						"name": "Computer monitors",
						"count": 1
					},
					{
						"id": "desktophd",
						"name": "Computer monitors larger than HD",
						"count": 0
					},
					{
						"id": "unknown",
						"name": "(unknown)",
						"count": 0
					}
				]
			}{
				"more": false,
				"stats": [
					{
						"name": "↔\ufe0e 1920px",
						"count": 1
					}
				]
			}`, `\ufe0e`, "\ufe0e"), list, get)
		}

		// Locations
		{
			var list HitStats
			err := list.ListLocations(ctx, rng, filter, 5, 0)
			if err != nil {
				t.Fatal(err)
			}
			var get HitStats
			err = get.ListLocation(ctx, "ID", rng, filter, 10, 0)
			if err != nil {
				t.Fatal(err)
			}

			// We don't have the cities db in tests, so it's expected to be
			// blank.
			err = zdb.Exec(ctx, `update locations set region_name='Noord-Brabant' where iso_3166_2='NL-NB';
				update locations set region_name='Bali' where iso_3166_2='ID-BA';`)
			if err != nil {
				t.Fatal(err)
			}
			var getRegion HitStats
			err = getRegion.ListLocation(ctx, "ID", rng, filter, 10, 0)
			if err != nil {
				t.Fatal(err)
			}

			cmp(t, `{
				"more": false,
				"stats": [
					{
						"id": "ID",
						"name": "Indonesia",
						"count": 1
					},
					{
						"id": "NL",
						"name": "The Netherlands",
						"count": 1
					}
				]
			}{
				"more": false,
				"stats": [
					{
						"name": "",
						"count": 1
					}
				]
			}{
				"more": false,
				"stats": [
					{
						"name": "Bali",
						"count": 1
					}
				]
			}`, list, get, getRegion)
		}
	}
}

func TestListSizes(t *testing.T) {
	ctx := gctest.DB(t)

	// Copy from hit_stats
	const (
		sizePhones      = "phone"
		sizeLargePhones = "largephone"
		sizeTablets     = "tablet"
		sizeDesktop     = "desktop"
		sizeDesktopHD   = "desktophd"
		sizeUnknown     = "unknown"
	)

	now := ztime.Now()
	widths := []struct {
		w  float64
		id string
	}{
		{0, sizeUnknown},
		{300, sizePhones},
		{1000, sizeLargePhones},
		{1100, sizeTablets},
		{1920, sizeDesktop},
		{3000, sizeDesktopHD},
	}

	for _, w := range widths {
		gctest.StoreHits(ctx, t, false,
			Hit{CreatedAt: now, Size: []float64{w.w, 0, 1}},
			Hit{CreatedAt: now, Size: []float64{w.w, 0, 1}, FirstVisit: true},
		)
	}
	gctest.StoreHits(ctx, t, false,
		Hit{CreatedAt: now, Size: []float64{4000, 0, 1}},
		Hit{CreatedAt: now, Size: []float64{4000, 0, 1}, FirstVisit: true},
		Hit{CreatedAt: now, Size: []float64{4200, 0, 1}},
		Hit{CreatedAt: now, Size: []float64{4200, 0, 1}, FirstVisit: true},
	)

	t.Run("ListSizes", func(t *testing.T) {
		var s HitStats
		err := s.ListSizes(ctx, ztime.NewRange(now).To(now), nil)
		if err != nil {
			t.Fatal(err)
		}

		got := string(zjson.MustMarshalIndent(s, "\t\t", "\t"))
		want := `{
			"more": false,
			"stats": [
				{
					"id": "phone",
					"name": "Phones",
					"count": 1
				},
				{
					"id": "largephone",
					"name": "Large phones, small tablets",
					"count": 1
				},
				{
					"id": "tablet",
					"name": "Tablets and small laptops",
					"count": 1
				},
				{
					"id": "desktop",
					"name": "Computer monitors",
					"count": 1
				},
				{
					"id": "desktophd",
					"name": "Computer monitors larger than HD",
					"count": 3
				},
				{
					"id": "unknown",
					"name": "(unknown)",
					"count": 1
				}
			]
		}`
		if d := ztest.Diff(got, want); d != "" {
			t.Error(d)
		}
	})

	t.Run("ListSize", func(t *testing.T) {
		var got string
		for _, w := range widths {
			var s HitStats
			err := s.ListSize(ctx, w.id, ztime.NewRange(now).To(now), nil, 10, 0)
			if err != nil {
				t.Fatal(err)
			}

			got += string(zjson.MustMarshalIndent(s, "\t\t", "\t"))
		}

		want := strings.ReplaceAll(`{
			"more": false,
			"stats": [
				{
					"name": "↔\ufe0e 0px",
					"count": 1
				}
			]
		}{
			"more": false,
			"stats": [
				{
					"name": "↔\ufe0e 300px",
					"count": 1
				}
			]
		}{
			"more": false,
			"stats": [
				{
					"name": "↔\ufe0e 1000px",
					"count": 1
				}
			]
		}{
			"more": false,
			"stats": [
				{
					"name": "↔\ufe0e 1100px",
					"count": 1
				}
			]
		}{
			"more": false,
			"stats": [
				{
					"name": "↔\ufe0e 1920px",
					"count": 1
				}
			]
		}{
			"more": false,
			"stats": [
				{
					"name": "↔\ufe0e 3000px",
					"count": 1
				},
				{
					"name": "↔\ufe0e 4000px",
					"count": 1
				},
				{
					"name": "↔\ufe0e 4200px",
					"count": 1
				}
			]
		}`, `\ufe0e`, "\ufe0e")
		if d := ztest.Diff(got, want); d != "" {
			t.Error(d)
		}
	})
}

func TestStatsByRef(t *testing.T) {
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/a", Ref: "https://example.com"},
		Hit{Path: "/b", Ref: "https://example.com"},
		Hit{Path: "/a", Ref: "https://example.org"})

	var have HitStats
	err := have.ListTopRef(ctx, "example.com", ztime.NewRange(ztime.Now().Add(-1*time.Hour)).To(ztime.Now().Add(1*time.Hour)),
		[]int64{1}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{
		"more": false,
		"stats": [{
			"count": 0,
			"name": "/a"
		}]
	}`
	if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
		t.Error(d)
	}
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"fmt"
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
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "Firefox",
						"Count": 3,
						"CountUnique": 2,
						"RefScheme": null
					}
				]
			}{
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "Firefox 79",
						"Count": 1,
						"CountUnique": 1,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "Firefox 81",
						"Count": 2,
						"CountUnique": 1,
						"RefScheme": null
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
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "Linux",
						"Count": 3,
						"CountUnique": 2,
						"RefScheme": null
					}
				]
			}{
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "Linux",
						"Count": 2,
						"CountUnique": 1,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "Linux Ubuntu",
						"Count": 1,
						"CountUnique": 1,
						"RefScheme": null
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
			err = get.ListSize(ctx, "Computer monitors", rng, filter, 10, 0)
			if err != nil {
				t.Fatal(err)
			}
			cmp(t, strings.ReplaceAll(`{
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "Phones",
						"Count": 0,
						"CountUnique": 0,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "Large phones, small tablets",
						"Count": 1,
						"CountUnique": 1,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "Tablets and small laptops",
						"Count": 0,
						"CountUnique": 0,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "Computer monitors",
						"Count": 2,
						"CountUnique": 1,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "Computer monitors larger than HD",
						"Count": 0,
						"CountUnique": 0,
						"RefScheme": null
					},
					{
						"ID": "",
						"Name": "(unknown)",
						"Count": 0,
						"CountUnique": 0,
						"RefScheme": null
					}
				]
			}{
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "↔\ufe0e 1920px",
						"Count": 2,
						"CountUnique": 1,
						"RefScheme": null
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
				"More": false,
				"Stats": [
					{
						"ID": "ID",
						"Name": "Indonesia",
						"Count": 1,
						"CountUnique": 1,
						"RefScheme": null
					},
					{
						"ID": "NL",
						"Name": "Netherlands",
						"Count": 2,
						"CountUnique": 1,
						"RefScheme": null
					}
				]
			}{
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "",
						"Count": 1,
						"CountUnique": 1,
						"RefScheme": null
					}
				]
			}{
				"More": false,
				"Stats": [
					{
						"ID": "",
						"Name": "Bali",
						"Count": 1,
						"CountUnique": 1,
						"RefScheme": null
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
		sizePhones      = "Phones"
		sizeLargePhones = "Large phones, small tablets"
		sizeTablets     = "Tablets and small laptops"
		sizeDesktop     = "Computer monitors"
		sizeDesktopHD   = "Computer monitors larger than HD"
		sizeUnknown     = "(unknown)"
	)

	now := ztime.Now()
	widths := []struct {
		w    float64
		name string
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
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "Phones",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "Large phones, small tablets",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "Tablets and small laptops",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "Computer monitors",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "Computer monitors larger than HD",
					"Count": 6,
					"CountUnique": 3,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "(unknown)",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
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
			err := s.ListSize(ctx, w.name, ztime.NewRange(now).To(now), nil, 10, 0)
			if err != nil {
				t.Fatal(err)
			}

			got += string(zjson.MustMarshalIndent(s, "\t\t", "\t"))
		}

		want := strings.ReplaceAll(`{
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "↔\ufe0e 0px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				}
			]
		}{
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "↔\ufe0e 300px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				}
			]
		}{
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "↔\ufe0e 1000px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				}
			]
		}{
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "↔\ufe0e 1100px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				}
			]
		}{
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "↔\ufe0e 1920px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				}
			]
		}{
			"More": false,
			"Stats": [
				{
					"ID": "",
					"Name": "↔\ufe0e 3000px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "↔\ufe0e 4000px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
				},
				{
					"ID": "",
					"Name": "↔\ufe0e 4200px",
					"Count": 2,
					"CountUnique": 1,
					"RefScheme": null
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

	var s HitStats
	err := s.ListTopRef(ctx, "example.com", ztime.NewRange(ztime.Now().Add(-1*time.Hour)).To(ztime.Now().Add(1*time.Hour)),
		[]int64{1}, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{false [{ /a 1 0 <nil>}]}`
	got := fmt.Sprintf("%v", s)
	if got != want {
		t.Fatalf("\nout:  %v\nwant: %v\n", got, want)
	}
}

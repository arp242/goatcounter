// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"fmt"
	"testing"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/ztest"
)

func TestLocations(t *testing.T) {
	ctx := gctest.DB(t)

	run := func() {
		{
			var l Location
			err := l.Lookup(ctx, "66.66.66.66")
			if err != nil {
				t.Fatal(err)
			}

			out := fmt.Sprintf("%#v", l)
			want := `goatcounter.Location{ID:2, Country:"US", Region:"", CountryName:"United States", RegionName:"", ISO3166_2:"US"}`
			if out != want {
				t.Error(out)
			}
		}
		{
			var l Location
			err := l.ByCode(ctx, "US-TX")
			if err != nil {
				t.Fatal(err)
			}

			out := fmt.Sprintf("%#v", l)
			want := `goatcounter.Location{ID:3, Country:"US", Region:"TX", CountryName:"United States", RegionName:"", ISO3166_2:"US-TX"}`
			if out != want {
				t.Error(out)
			}
		}

		out := zdb.DumpString(ctx, `select * from locations`)
		want := `
			location_id  iso_3166_2  country  region  country_name   region_name
			1                                         (unknown)
			2            US          US               United States
			3            US-TX       US       TX      United States`
		if d := ztest.Diff(out, want, ztest.DiffNormalizeWhitespace); d != "" {
			t.Error(d)
		}
	}

	// Run it multiple times, since it should always give the same resuts.
	run()
	run()
	ctx = NewContext(zdb.MustGetDB(ctx)) // Reset cache
	run()
}

func BenchmarkLocationsByCode(b *testing.B) {
	ctx := gctest.DB(b)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		(&Location{}).ByCode(ctx, "US-TX")
	}
}

package goatcounter_test

import (
	"fmt"
	"testing"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/goatcounter/v2/pkg/geo"
	"zgo.at/zdb"
	"zgo.at/zstd/ztest"
)

func TestLocations(t *testing.T) {
	geodb, _ := geo.Open("")
	ctx := geo.With(gctest.DB(t), geodb)

	run := func() {
		{
			var l Location
			err := l.Lookup(ctx, "51.171.91.33")
			if err != nil {
				t.Fatal(err)
			}

			out := fmt.Sprintf("%#v", l)
			want := `goatcounter.Location{ID:2, Country:"IE", Region:"", CountryName:"Ireland", RegionName:"", ISO3166_2:"IE"}`
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
			2            IE          IE               Ireland
			3            US-TX       US       TX      United States
			4            US          US               United States`
		if d := ztest.Diff(out, want, ztest.DiffNormalizeWhitespace); d != "" {
			t.Error(d)
		}
	}

	// Run it multiple times, since it should always give the same resuts.
	run()
	run()
	ctx = NewContext(ctx, zdb.MustGetDB(ctx)) // Reset cache
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

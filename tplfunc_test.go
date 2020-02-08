// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/tz"
	"zgo.at/ztest"
)

func TestBarChart(t *testing.T) {
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
	tests := []struct {
		zone string
		now  time.Time
		want string
	}{
		{"Europe/London", now, `
			<div title="2019-08-31 0:00 – 0:59, 0 views"></div>
			<div title="2019-08-31 1:00 – 1:59, 0 views"></div>
			<div title="2019-08-31 2:00 – 2:59, 0 views"></div>
			<div title="2019-08-31 3:00 – 3:59, 0 views"></div>
			<div title="2019-08-31 4:00 – 4:59, 0 views"></div>
			<div title="2019-08-31 5:00 – 5:59, 0 views"></div>
			<div title="2019-08-31 6:00 – 6:59, 0 views"></div>
			<div title="2019-08-31 7:00 – 7:59, 0 views"></div>
			<div title="2019-08-31 8:00 – 8:59, 0 views"></div>
			<div title="2019-08-31 9:00 – 9:59, 0 views"></div>
			<div title="2019-08-31 10:00 – 10:59, 0 views"></div>
			<div title="2019-08-31 11:00 – 11:59, 0 views"></div>
			<div title="2019-08-31 12:00 – 12:59, 0 views"></div>
			<div title="2019-08-31 13:00 – 13:59, 0 views"></div>
			<div title="2019-08-31 14:00 – 14:59, 1 views"><div style="height: 10%;"></div>
			</div>
			<div title="2019-08-31 15:00 – 15:59, 0 views"></div>
			<div title="2019-08-31 16:00 – 16:59, 0 views"></div>
			<div title="2019-08-31 17:00 – 17:59, 0 views"></div>
			<div title="2019-08-31 18:00 – 18:59, 0 views"></div>
			<div title="2019-08-31 19:00 – 19:59, 0 views"></div>
			<div title="2019-08-31 20:00 – 20:59, 0 views"></div>
			<div title="2019-08-31 21:00 – 21:59, 0 views"></div>
			<div title="2019-08-31 22:00 – 22:59, 0 views"></div>
			<div title="2019-08-31 23:00 – 23:59, 0 views"></div>
		`},
		{"Asia/Makassar", now, `
			<div title="2019-08-31 0:00 – 0:59, 0 views"></div>
			<div title="2019-08-31 1:00 – 1:59, 0 views"></div>
			<div title="2019-08-31 2:00 – 2:59, 0 views"></div>
			<div title="2019-08-31 3:00 – 3:59, 0 views"></div>
			<div title="2019-08-31 4:00 – 4:59, 0 views"></div>
			<div title="2019-08-31 5:00 – 5:59, 0 views"></div>
			<div title="2019-08-31 6:00 – 6:59, 0 views"></div>
			<div title="2019-08-31 7:00 – 7:59, 0 views"></div>
			<div title="2019-08-31 8:00 – 8:59, 0 views"></div>
			<div title="2019-08-31 9:00 – 9:59, 0 views"></div>
			<div title="2019-08-31 10:00 – 10:59, 0 views"></div>
			<div title="2019-08-31 11:00 – 11:59, 0 views"></div>
			<div title="2019-08-31 12:00 – 12:59, 0 views"></div>
			<div title="2019-08-31 13:00 – 13:59, 0 views"></div>
			<div title="2019-08-31 14:00 – 14:59, 0 views"></div>
			<div title="2019-08-31 15:00 – 15:59, 0 views"></div>
			<div title="2019-08-31 16:00 – 16:59, 0 views"></div>
			<div title="2019-08-31 17:00 – 17:59, 0 views"></div>
			<div title="2019-08-31 18:00 – 18:59, 0 views"></div>
			<div title="2019-08-31 19:00 – 19:59, 0 views"></div>
			<div title="2019-08-31 20:00 – 20:59, 0 views"></div>
			<div title="2019-08-31 21:00 – 21:59, 0 views"></div>
			<div title="2019-08-31 22:00 – 22:59, 1 views"><div style="height: 10%;"></div>
			</div>
			<div title="2019-08-31 23:00 – 23:59, 0 views"></div>
		`},
		{"Pacific/Auckland", now, `
			<div title="2019-08-31 0:00 – 0:59, 0 views"></div>
			<div title="2019-08-31 1:00 – 1:59, 0 views"></div>
			<div title="2019-08-31 2:00 – 2:59, 0 views"></div>
			<div title="2019-08-31 3:00 – 3:59, 0 views"></div>
			<div title="2019-08-31 4:00 – 4:59, 0 views"></div>
			<div title="2019-08-31 5:00 – 5:59, 0 views"></div>
			<div title="2019-08-31 6:00 – 6:59, 0 views"></div>
			<div title="2019-08-31 7:00 – 7:59, 0 views"></div>
			<div title="2019-08-31 8:00 – 8:59, 0 views"></div>
			<div title="2019-08-31 9:00 – 9:59, 0 views"></div>
			<div title="2019-08-31 10:00 – 10:59, 0 views"></div>
			<div title="2019-08-31 11:00 – 11:59, 0 views"></div>
			<div title="2019-08-31 12:00 – 12:59, 0 views"></div>
			<div title="2019-08-31 13:00 – 13:59, 0 views"></div>
			<div title="2019-08-31 14:00 – 14:59, 0 views"></div>
			<div title="2019-08-31 15:00 – 15:59, 0 views"></div>
			<div title="2019-08-31 16:00 – 16:59, 0 views"></div>
			<div title="2019-08-31 17:00 – 17:59, 0 views"></div>
			<div title="2019-08-31 18:00 – 18:59, 0 views"></div>
			<div title="2019-08-31 19:00 – 19:59, 0 views"></div>
			<div title="2019-08-31 20:00 – 20:59, 0 views"></div>
			<div title="2019-08-31 21:00 – 21:59, 0 views"></div>
			<div title="2019-08-31 22:00 – 22:59, 0 views"></div>
			<div title="2019-08-31 23:00 – 23:59, 0 views"></div>
		`},
		{"Pacific/Pago_Pago", time.Date(2019, 8, 31, 9, 42, 0, 0, time.UTC), `
			<div title="2019-08-31 0:00 – 0:59, 0 views"></div>
			<div title="2019-08-31 1:00 – 1:59, 0 views"></div>
			<div title="2019-08-31 2:00 – 2:59, 0 views"></div>
			<div title="2019-08-31 3:00 – 3:59, 0 views"></div>
			<div title="2019-08-31 4:00 – 4:59, 0 views"></div>
			<div title="2019-08-31 5:00 – 5:59, 0 views"></div>
			<div title="2019-08-31 6:00 – 6:59, 0 views"></div>
			<div title="2019-08-31 7:00 – 7:59, 0 views"></div>
			<div title="2019-08-31 8:00 – 8:59, 0 views"></div>
			<div title="2019-08-31 9:00 – 9:59, 0 views"></div>
			<div title="2019-08-31 10:00 – 10:59, 0 views"></div>
			<div title="2019-08-31 11:00 – 11:59, 0 views"></div>
			<div title="2019-08-31 12:00 – 12:59, 0 views"></div>
			<div title="2019-08-31 13:00 – 13:59, 0 views"></div>
			<div title="2019-08-31 14:00 – 14:59, 0 views"></div>
			<div title="2019-08-31 15:00 – 15:59, 0 views"></div>
			<div title="2019-08-31 16:00 – 16:59, 0 views"></div>
			<div title="2019-08-31 17:00 – 17:59, 0 views"></div>
			<div title="2019-08-31 18:00 – 18:59, 0 views"></div>
			<div title="2019-08-31 19:00 – 19:59, 0 views"></div>
			<div title="2019-08-31 20:00 – 20:59, 0 views"></div>
			<div title="2019-08-31 21:00 – 21:59, 0 views"></div>
			<div title="2019-08-31 22:00 – 22:59, 0 views"></div>
			<div title="2019-08-31 23:00 – 23:59, 0 views"></div>
		`},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {

			ctx, clean := gctest.DB(t)
			defer clean()

			zone, err := tz.New("", tt.zone)
			if err != nil {
				t.Fatal(err)
			}

			ctx, site := gctest.Site(ctx, t, Site{
				CreatedAt: tt.now,
				Settings:  SiteSettings{Timezone: zone},
			})

			gctest.StoreHits(ctx, t, Hit{Site: site.ID, CreatedAt: tt.now, Path: "/a"})

			var stats HitStats
			_, _, _, err = stats.List(ctx, tt.now, tt.now, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(stats) != 1 {
				t.Fatalf("len(stats) == %d", len(stats))
			}

			out := string(BarChart(ctx, stats[0].Stats, stats[0].Max))
			out = strings.TrimSpace(strings.ReplaceAll(out, "</div>", "</div>\n"))
			tt.want = strings.TrimSpace(strings.ReplaceAll(tt.want, "\t", ""))

			if d := ztest.Diff(strings.Split(out, "\n"), strings.Split(tt.want, "\n")); d != "" {
				t.Error(d)
			}
		})
	}
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztest"
)

func TestDashboard(t *testing.T) {
	tests := []handlerTest{
		{
			name:     "no-data",
			router:   newBackend,
			auth:     true,
			wantCode: 200,
			wantBody: "<strong>No data received</strong>",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestDashboardBarChart(t *testing.T) {
	id := tz.MustNew("", "Asia/Makassar").Loc()
	hi := tz.MustNew("", "Pacific/Honolulu").Loc()

	type testcase struct {
		zone                  string
		now, hit              time.Time
		wantHourly, wantDaily string
		wantText              string
		wantNothing           bool
	}

	// The requested time is always from 2019-06-17 to 2019-06-18, in the local
	// TZ.
	tests := []testcase{
		{
			zone: "UTC",
			now:  date("2019-06-18 14:43", time.UTC),
			hit:  date("2019-06-18 12:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|1|0"></div>`,
			wantHourly: `
				<div title="2019-06-17|0:00|0:59|0|0"></div>
				<div title="2019-06-17|1:00|1:59|0|0"></div>
				<div title="2019-06-17|2:00|2:59|0|0"></div>
				<div title="2019-06-17|3:00|3:59|0|0"></div>
				<div title="2019-06-17|4:00|4:59|0|0"></div>
				<div title="2019-06-17|5:00|5:59|0|0"></div>
				<div title="2019-06-17|6:00|6:59|0|0"></div>
				<div title="2019-06-17|7:00|7:59|0|0"></div>
				<div title="2019-06-17|8:00|8:59|0|0"></div>
				<div title="2019-06-17|9:00|9:59|0|0"></div>
				<div title="2019-06-17|10:00|10:59|0|0"></div>
				<div title="2019-06-17|11:00|11:59|0|0"></div>
				<div title="2019-06-17|12:00|12:59|0|0"></div>
				<div title="2019-06-17|13:00|13:59|0|0"></div>
				<div title="2019-06-17|14:00|14:59|0|0"></div>
				<div title="2019-06-17|15:00|15:59|0|0"></div>
				<div title="2019-06-17|16:00|16:59|0|0"></div>
				<div title="2019-06-17|17:00|17:59|0|0"></div>
				<div title="2019-06-17|18:00|18:59|0|0"></div>
				<div title="2019-06-17|19:00|19:59|0|0"></div>
				<div title="2019-06-17|20:00|20:59|0|0"></div>
				<div title="2019-06-17|21:00|21:59|0|0"></div>
				<div title="2019-06-17|22:00|22:59|0|0"></div>
				<div title="2019-06-17|23:00|23:59|0|0"></div>
				<div title="2019-06-18|0:00|0:59|0|0"></div>
				<div title="2019-06-18|1:00|1:59|0|0"></div>
				<div title="2019-06-18|2:00|2:59|0|0"></div>
				<div title="2019-06-18|3:00|3:59|0|0"></div>
				<div title="2019-06-18|4:00|4:59|0|0"></div>
				<div title="2019-06-18|5:00|5:59|0|0"></div>
				<div title="2019-06-18|6:00|6:59|0|0"></div>
				<div title="2019-06-18|7:00|7:59|0|0"></div>
				<div title="2019-06-18|8:00|8:59|0|0"></div>
				<div title="2019-06-18|9:00|9:59|0|0"></div>
				<div title="2019-06-18|10:00|10:59|0|0"></div>
				<div title="2019-06-18|11:00|11:59|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|12:00|12:59|1|0"></div>
				<div title="2019-06-18|13:00|13:59|0|0"></div>
				<div title="2019-06-18|14:00|14:59|0|0"></div>`, // Future not displayed
		},

		// +8
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 14:42", time.UTC),
			hit:  date("2019-06-18 12:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|1|0"></div>`,
			wantHourly: `
				<div title="2019-06-17|0:00|0:59|0|0"></div>
				<div title="2019-06-17|1:00|1:59|0|0"></div>
				<div title="2019-06-17|2:00|2:59|0|0"></div>
				<div title="2019-06-17|3:00|3:59|0|0"></div>
				<div title="2019-06-17|4:00|4:59|0|0"></div>
				<div title="2019-06-17|5:00|5:59|0|0"></div>
				<div title="2019-06-17|6:00|6:59|0|0"></div>
				<div title="2019-06-17|7:00|7:59|0|0"></div>
				<div title="2019-06-17|8:00|8:59|0|0"></div>
				<div title="2019-06-17|9:00|9:59|0|0"></div>
				<div title="2019-06-17|10:00|10:59|0|0"></div>
				<div title="2019-06-17|11:00|11:59|0|0"></div>
				<div title="2019-06-17|12:00|12:59|0|0"></div>
				<div title="2019-06-17|13:00|13:59|0|0"></div>
				<div title="2019-06-17|14:00|14:59|0|0"></div>
				<div title="2019-06-17|15:00|15:59|0|0"></div>
				<div title="2019-06-17|16:00|16:59|0|0"></div>
				<div title="2019-06-17|17:00|17:59|0|0"></div>
				<div title="2019-06-17|18:00|18:59|0|0"></div>
				<div title="2019-06-17|19:00|19:59|0|0"></div>
				<div title="2019-06-17|20:00|20:59|0|0"></div>
				<div title="2019-06-17|21:00|21:59|0|0"></div>
				<div title="2019-06-17|22:00|22:59|0|0"></div>
				<div title="2019-06-17|23:00|23:59|0|0"></div>
				<div title="2019-06-18|0:00|0:59|0|0"></div>
				<div title="2019-06-18|1:00|1:59|0|0"></div>
				<div title="2019-06-18|2:00|2:59|0|0"></div>
				<div title="2019-06-18|3:00|3:59|0|0"></div>
				<div title="2019-06-18|4:00|4:59|0|0"></div>
				<div title="2019-06-18|5:00|5:59|0|0"></div>
				<div title="2019-06-18|6:00|6:59|0|0"></div>
				<div title="2019-06-18|7:00|7:59|0|0"></div>
				<div title="2019-06-18|8:00|8:59|0|0"></div>
				<div title="2019-06-18|9:00|9:59|0|0"></div>
				<div title="2019-06-18|10:00|10:59|0|0"></div>
				<div title="2019-06-18|11:00|11:59|0|0"></div>
				<div title="2019-06-18|12:00|12:59|0|0"></div>
				<div title="2019-06-18|13:00|13:59|0|0"></div>
				<div title="2019-06-18|14:00|14:59|0|0"></div>
				<div title="2019-06-18|15:00|15:59|0|0"></div>
				<div title="2019-06-18|16:00|16:59|0|0"></div>
				<div title="2019-06-18|17:00|17:59|0|0"></div>
				<div title="2019-06-18|18:00|18:59|0|0"></div>
				<div title="2019-06-18|19:00|19:59|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|20:00|20:59|1|0"></div>
				<div title="2019-06-18|21:00|21:59|0|0"></div>
				<div title="2019-06-18|22:00|22:59|0|0"></div>`,
		},

		// in the future, so nothing displayed.
		// {
		// 	zone:        "Asia/Makassar",
		// 	now:         date("2019-06-18 14:42", time.UTC),
		// 	hit:         date("2019-06-18 23:42", time.UTC),
		// 	wantNothing: true,
		// },

		// The hit is added on the 17th, but displayed on the 18th
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 2:16", id),
			hit:  date("2019-06-17 18:15", time.UTC),
			wantDaily: `
				<div title="2019-06-17|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|1|0"></div>`,
			wantHourly: `
				<div title="2019-06-17|0:00|0:59|0|0"></div>
				<div title="2019-06-17|1:00|1:59|0|0"></div>
				<div title="2019-06-17|2:00|2:59|0|0"></div>
				<div title="2019-06-17|3:00|3:59|0|0"></div>
				<div title="2019-06-17|4:00|4:59|0|0"></div>
				<div title="2019-06-17|5:00|5:59|0|0"></div>
				<div title="2019-06-17|6:00|6:59|0|0"></div>
				<div title="2019-06-17|7:00|7:59|0|0"></div>
				<div title="2019-06-17|8:00|8:59|0|0"></div>
				<div title="2019-06-17|9:00|9:59|0|0"></div>
				<div title="2019-06-17|10:00|10:59|0|0"></div>
				<div title="2019-06-17|11:00|11:59|0|0"></div>
				<div title="2019-06-17|12:00|12:59|0|0"></div>
				<div title="2019-06-17|13:00|13:59|0|0"></div>
				<div title="2019-06-17|14:00|14:59|0|0"></div>
				<div title="2019-06-17|15:00|15:59|0|0"></div>
				<div title="2019-06-17|16:00|16:59|0|0"></div>
				<div title="2019-06-17|17:00|17:59|0|0"></div>
				<div title="2019-06-17|18:00|18:59|0|0"></div>
				<div title="2019-06-17|19:00|19:59|0|0"></div>
				<div title="2019-06-17|20:00|20:59|0|0"></div>
				<div title="2019-06-17|21:00|21:59|0|0"></div>
				<div title="2019-06-17|22:00|22:59|0|0"></div>
				<div title="2019-06-17|23:00|23:59|0|0"></div>
				<div title="2019-06-18|0:00|0:59|0|0"></div>
				<div title="2019-06-18|1:00|1:59|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|2:00|2:59|1|0"></div>`,
		},

		// The hit is added on the 16th, but displayed on the 17th
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 2:16", id),
			hit:  date("2019-06-16 18:15", time.UTC),
			wantDaily: `
				<div style="height:10%" data-u="0%" title="2019-06-17|1|0"></div>
				<div title="2019-06-18|0|0"></div>`,
			wantHourly: `
				<div title="2019-06-17|0:00|0:59|0|0"></div>
				<div title="2019-06-17|1:00|1:59|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-17|2:00|2:59|1|0"></div>
				<div title="2019-06-17|3:00|3:59|0|0"></div>
				<div title="2019-06-17|4:00|4:59|0|0"></div>
				<div title="2019-06-17|5:00|5:59|0|0"></div>
				<div title="2019-06-17|6:00|6:59|0|0"></div>
				<div title="2019-06-17|7:00|7:59|0|0"></div>
				<div title="2019-06-17|8:00|8:59|0|0"></div>
				<div title="2019-06-17|9:00|9:59|0|0"></div>
				<div title="2019-06-17|10:00|10:59|0|0"></div>
				<div title="2019-06-17|11:00|11:59|0|0"></div>
				<div title="2019-06-17|12:00|12:59|0|0"></div>
				<div title="2019-06-17|13:00|13:59|0|0"></div>
				<div title="2019-06-17|14:00|14:59|0|0"></div>
				<div title="2019-06-17|15:00|15:59|0|0"></div>
				<div title="2019-06-17|16:00|16:59|0|0"></div>
				<div title="2019-06-17|17:00|17:59|0|0"></div>
				<div title="2019-06-17|18:00|18:59|0|0"></div>
				<div title="2019-06-17|19:00|19:59|0|0"></div>
				<div title="2019-06-17|20:00|20:59|0|0"></div>
				<div title="2019-06-17|21:00|21:59|0|0"></div>
				<div title="2019-06-17|22:00|22:59|0|0"></div>
				<div title="2019-06-17|23:00|23:59|0|0"></div>
				<div title="2019-06-18|0:00|0:59|0|0"></div>
				<div title="2019-06-18|1:00|1:59|0|0"></div>
				<div title="2019-06-18|2:00|2:59|0|0"></div>`,
		},

		// -10
		{
			zone: "Pacific/Honolulu",
			now:  date("2019-06-18 14:42", time.UTC),
			hit:  date("2019-06-18 12:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|1|0"></div>`,
			wantHourly: `
				<div title="2019-06-17|0:00|0:59|0|0"></div>
				<div title="2019-06-17|1:00|1:59|0|0"></div>
				<div title="2019-06-17|2:00|2:59|0|0"></div>
				<div title="2019-06-17|3:00|3:59|0|0"></div>
				<div title="2019-06-17|4:00|4:59|0|0"></div>
				<div title="2019-06-17|5:00|5:59|0|0"></div>
				<div title="2019-06-17|6:00|6:59|0|0"></div>
				<div title="2019-06-17|7:00|7:59|0|0"></div>
				<div title="2019-06-17|8:00|8:59|0|0"></div>
				<div title="2019-06-17|9:00|9:59|0|0"></div>
				<div title="2019-06-17|10:00|10:59|0|0"></div>
				<div title="2019-06-17|11:00|11:59|0|0"></div>
				<div title="2019-06-17|12:00|12:59|0|0"></div>
				<div title="2019-06-17|13:00|13:59|0|0"></div>
				<div title="2019-06-17|14:00|14:59|0|0"></div>
				<div title="2019-06-17|15:00|15:59|0|0"></div>
				<div title="2019-06-17|16:00|16:59|0|0"></div>
				<div title="2019-06-17|17:00|17:59|0|0"></div>
				<div title="2019-06-17|18:00|18:59|0|0"></div>
				<div title="2019-06-17|19:00|19:59|0|0"></div>
				<div title="2019-06-17|20:00|20:59|0|0"></div>
				<div title="2019-06-17|21:00|21:59|0|0"></div>
				<div title="2019-06-17|22:00|22:59|0|0"></div>
				<div title="2019-06-17|23:00|23:59|0|0"></div>
				<div title="2019-06-18|0:00|0:59|0|0"></div>
				<div title="2019-06-18|1:00|1:59|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-18|2:00|2:59|1|0"></div>
				<div title="2019-06-18|3:00|3:59|0|0"></div>
				<div title="2019-06-18|4:00|4:59|0|0"></div>`,
		},

		// The hit is added on the 18th, but displayed on the 17th
		{
			zone: "Pacific/Honolulu",
			now:  date("2019-06-18 14:42", hi),
			hit:  date("2019-06-18 2:42", time.UTC),
			wantDaily: `
				<div style="height:10%" data-u="0%" title="2019-06-17|1|0"></div>
				<div title="2019-06-18|0|0"></div>`,
			wantHourly: `
				<div title="2019-06-17|0:00|0:59|0|0"></div>
				<div title="2019-06-17|1:00|1:59|0|0"></div>
				<div title="2019-06-17|2:00|2:59|0|0"></div>
				<div title="2019-06-17|3:00|3:59|0|0"></div>
				<div title="2019-06-17|4:00|4:59|0|0"></div>
				<div title="2019-06-17|5:00|5:59|0|0"></div>
				<div title="2019-06-17|6:00|6:59|0|0"></div>
				<div title="2019-06-17|7:00|7:59|0|0"></div>
				<div title="2019-06-17|8:00|8:59|0|0"></div>
				<div title="2019-06-17|9:00|9:59|0|0"></div>
				<div title="2019-06-17|10:00|10:59|0|0"></div>
				<div title="2019-06-17|11:00|11:59|0|0"></div>
				<div title="2019-06-17|12:00|12:59|0|0"></div>
				<div title="2019-06-17|13:00|13:59|0|0"></div>
				<div title="2019-06-17|14:00|14:59|0|0"></div>
				<div title="2019-06-17|15:00|15:59|0|0"></div>
				<div style="height:10%" data-u="0%" title="2019-06-17|16:00|16:59|1|0"></div>
				<div title="2019-06-17|17:00|17:59|0|0"></div>
				<div title="2019-06-17|18:00|18:59|0|0"></div>
				<div title="2019-06-17|19:00|19:59|0|0"></div>
				<div title="2019-06-17|20:00|20:59|0|0"></div>
				<div title="2019-06-17|21:00|21:59|0|0"></div>
				<div title="2019-06-17|22:00|22:59|0|0"></div>
				<div title="2019-06-17|23:00|23:59|0|0"></div>
				<div title="2019-06-18|0:00|0:59|0|0"></div>
				<div title="2019-06-18|1:00|1:59|0|0"></div>
				<div title="2019-06-18|2:00|2:59|0|0"></div>
				<div title="2019-06-18|3:00|3:59|0|0"></div>
				<div title="2019-06-18|4:00|4:59|0|0"></div>
				<div title="2019-06-18|5:00|5:59|0|0"></div>
				<div title="2019-06-18|6:00|6:59|0|0"></div>
				<div title="2019-06-18|7:00|7:59|0|0"></div>
				<div title="2019-06-18|8:00|8:59|0|0"></div>
				<div title="2019-06-18|9:00|9:59|0|0"></div>
				<div title="2019-06-18|10:00|10:59|0|0"></div>
				<div title="2019-06-18|11:00|11:59|0|0"></div>
				<div title="2019-06-18|12:00|12:59|0|0"></div>
				<div title="2019-06-18|13:00|13:59|0|0"></div>
				<div title="2019-06-18|14:00|14:59|0|0"></div>`,
		},
	}

	run := func(t *testing.T, tt testcase, url, want string) {
		ctx, clean := gctest.DB(t)
		defer clean()

		ctx, site := gctest.Site(ctx, t, goatcounter.Site{
			CreatedAt: time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC),
			Settings:  goatcounter.SiteSettings{Timezone: tz.MustNew("", tt.zone)},
		})

		gctest.StoreHits(ctx, t, false, goatcounter.Hit{
			Site:      site.ID,
			CreatedAt: tt.hit.UTC(),
			Path:      "/a",
		})

		t.Run("text", func(t *testing.T) {
			r, rr := newTest(ctx, "GET", url+"&as-text=on", nil)
			r.Host = site.Code + "." + goatcounter.Config(ctx).Domain
			login(t, r)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 200)

			doc, err := goquery.NewDocumentFromReader(rr.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantNothing {
				// TODO: test this
				return
			}

			t.Run("pages", func(t *testing.T) {
				c := doc.Find(".pages")
				if c.Length() != 1 {
					t.Fatalf("c.Length: %d", c.Length())
				}

				out, err := goquery.OuterHtml(c.Find("tr"))
				if err != nil {
					t.Fatal(err)
				}

				out = strings.TrimSpace(regexp.MustCompile(`[ \t]+<`).ReplaceAllString(out, "<"))
				out = strings.TrimSpace(regexp.MustCompile(`[ \t]+`).ReplaceAllString(out, " "))
				out = regexp.MustCompile(`(?m)^\s+$`).ReplaceAllString(out, "")
				out = regexp.MustCompile("\n+").ReplaceAllString(out, "\n")

				// want := strings.TrimSpace(strings.ReplaceAll(tt.wantText, "\t", ""))
				want := strings.TrimSpace(strings.ReplaceAll(`
					<tr id="/a" data-id="1" class=" ">
					<td class="col-idx">1</td>
					<td class="col-n col-count">0</td>
					<td class="col-n">1</td>
					<td class="col-p">
					<a class="load-refs rlink" href="#">/a</a>
					<div class="refs hchart" data-more="/hchart-more?kind=ref">
					</div>
					</td>
					<td class="col-t"><em>(no title)</em>
					</td>
					<td class="col-d"><span>            </span></td>
					</tr>`, "\t", ""))

				if d := ztest.Diff(out, want); d != "" {
					t.Error(d)
					fmt.Println("\n" + out)
				}
			})

			t.Run("totals", func(t *testing.T) {
				// TODO: not implemented yet.
			})
		})

		t.Run("standard", func(t *testing.T) {
			r, rr := newTest(ctx, "GET", url, nil)
			r.Host = site.Code + "." + goatcounter.Config(ctx).Domain
			login(t, r)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 200)

			doc, err := goquery.NewDocumentFromReader(rr.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantNothing {
				// TODO: test this
				return
			}

			cleanChart := func(h string) string {
				h = strings.ReplaceAll(h, "</div>", "</div>\n")
				h = strings.ReplaceAll(h, "</div>\n</div>", "</div></div>")
				return strings.TrimSpace(regexp.MustCompile(`[ \t]+<`).ReplaceAllString(h, "<"))
			}

			t.Run("pages", func(t *testing.T) {
				c := doc.Find(".pages-list .chart.chart-bar")
				if c.Length() != 1 {
					t.Fatalf("c.Length: %d", c.Length())
				}
				chart, err := c.Eq(0).Html()
				if err != nil {
					t.Fatal(err)
				}
				chart = cleanChart(chart)

				want := `` +
					`<span class="chart-left"><a href="#" class="rescale" title="Scale Y axis to max">` + "↕\ufe0e" + `</a></span>` + "\n" +
					`<span class="chart-right"><small class="scale" title="Y-axis scale">10</small></span>` + "\n" +
					`<span class="half"></span>` + "\n" +
					strings.TrimSpace(strings.ReplaceAll(want, "\t", ""))

				if d := ztest.Diff(chart, want); d != "" {
					t.Error(d)
					if zstring.Contains(os.Args, "-test.v=true") {
						fmt.Println("pages:\n" + chart)
					}
				}
			})

			t.Run("totals", func(t *testing.T) {
				c := doc.Find(".totals .chart.chart-bar")
				if c.Length() != 1 {
					t.Fatalf("c.Length: %d", c.Length())
				}
				chart, err := c.Eq(0).Html()
				if err != nil {
					t.Fatal(err)
				}
				chart = cleanChart(chart)

				want := `` +
					`<span class="chart-right"><small class="scale" title="Y-axis scale">10</small></span>` + "\n" +
					`<span class="half"></span>` + "\n" +
					strings.TrimSpace(strings.ReplaceAll(want, "\t", ""))

				if d := ztest.Diff(chart, want); d != "" {
					t.Error(d)
					if zstring.Contains(os.Args, "-test.v=true") {
						fmt.Println("totals:\n" + chart)
					}
				}
			})
		})
	}

	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			defer gctest.SwapNow(t, tt.now.UTC())()

			t.Run("hourly", func(t *testing.T) {
				run(t, tt, "/?period-start=2019-06-17&period-end=2019-06-18", tt.wantHourly)
			})
			t.Run("daily", func(t *testing.T) {
				run(t, tt, "/?period-start=2019-06-17&period-end=2019-06-18&daily=true", tt.wantDaily)
			})
		})
	}
}

func TestTimeRange(t *testing.T) {
	tests := []struct {
		rng, now, wantStart, wantEnd string
	}{
		{"week", "2020-12-02",
			"2020-11-25 00:00:00", "2020-12-02 23:59:59"},
		{"month", "2020-01-18",
			"2019-12-18 00:00:00", "2020-01-18 23:59:59"},
		{"quarter", "2020-01-18",
			"2019-10-18 00:00:00", "2020-01-18 23:59:59"},
		{"half-year", "2020-01-18",
			"2019-07-18 00:00:00", "2020-01-18 23:59:59"},
		{"year", "2020-01-18",
			"2019-01-18 00:00:00", "2020-01-18 23:59:59"},

		// TODO: also test with sundayStartsWeek
		{"week-cur", "2020-01-01",
			"2019-12-30 00:00:00", "2020-01-05 23:59:59"},

		{"month-cur", "2020-01-01",
			"2020-01-01 00:00:00", "2020-01-31 23:59:59"},
		{"month-cur", "2020-01-31",
			"2020-01-01 00:00:00", "2020-01-31 23:59:59"},

		{"0", "2020-06-18",
			"2020-06-18 00:00:00", "2020-06-18 23:59:59"},
		{"1", "2020-06-18",
			"2020-06-17 00:00:00", "2020-06-18 23:59:59"},
		{"42", "2020-06-18",
			"2020-05-07 00:00:00", "2020-06-18 23:59:59"},
	}

	for _, tt := range tests {
		t.Run(tt.rng+"-"+tt.now, func(t *testing.T) {
			defer gctest.SwapNow(t, tt.now)()

			t.Run("UTC", func(t *testing.T) {
				start, end, err := timeRange(tt.rng, time.UTC, false)
				if err != nil {
					t.Fatal(err)
				}

				gotStart := start.Format("2006-01-02 15:04:05")
				gotEnd := end.Format("2006-01-02 15:04:05")

				if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
					t.Errorf("\ngot:  %q, %q\nwant: %q, %q",
						gotStart, gotEnd, tt.wantStart, tt.wantEnd)
				}
			})

			// t.Run("Asia/Makassar", func(t *testing.T) {
			// })
		})
	}
}

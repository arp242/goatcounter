// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
	"zgo.at/tz"
	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/ztest"
)

func TestBackendCount(t *testing.T) {
	t.Skip() // TODO: these need to set query params, instead of body

	tests := []handlerTest{
		{
			name:         "basic",
			router:       newBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html"},
			wantCode:     200,
			wantFormCode: 200,
		},
		{
			name:         "params",
			router:       newBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html?param=xxx"},
			wantCode:     200,
			wantFormCode: 200,
		},

		{
			name:         "ref",
			router:       newBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com"},
			wantCode:     200,
			wantFormCode: 200,
		},
		{
			name:         "ref_params",
			router:       newBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com?p=xxx"},
			wantCode:     200,
			wantFormCode: 200,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			_, err := goatcounter.Memstore.Persist(r.Context())
			if err != nil {
				t.Fatal(err)
			}

			var hits []goatcounter.Hit
			err = zdb.MustGet(r.Context()).SelectContext(r.Context(), &hits, `select * from hits`)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) != 1 {
				t.Fatalf("len(hits) = %d: %#v", len(hits), hits)
			}

			h := hits[0]
			err = h.Validate(r.Context())
			if err != nil {
				t.Errorf("Validate failed after get: %s", err)
			}
		})
	}
}

func newBackend(db zdb.DB) chi.Router {
	return NewBackend(db, nil)
}

func TestBackendIndex(t *testing.T) {
	tests := []handlerTest{
		{
			name:     "no-data",
			router:   newBackend,
			auth:     true,
			wantCode: 200,
			wantBody: "<strong>No data received</strong>",
		},

		{
			name: "basic",
			setup: func(ctx context.Context) {
				goatcounter.Memstore.Append(goatcounter.Hit{Path: "/asdfghjkl", Site: 1})
				//_, err := goatcounter.Memstore.Persist(ctx)
				//if err != nil {
				//	panic(err)
				//}
				cron.RunOnce(zdb.MustGet(ctx))
			},
			router:   newBackend,
			auth:     true,
			wantCode: 200,
			// TODO: why 0 displayed?
			// <h2>Pages <sup>(total 1 hits, <span class="total-display">0</span> displayed)</sup></h2>
			//wantBody: "<h2>Pages <sup>(total 1 hits)</sup></h2>",
			//wantBody: `<span class="total-hits">1</span>`,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestBackendExport(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				goatcounter.Memstore.Append([]goatcounter.Hit{
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/zxc", CreatedAt: now},
				}...)
				_, err := goatcounter.Memstore.Persist(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:   newBackend,
			path:     "/export/hits.csv",
			auth:     true,
			wantCode: 200,
			wantBody: "/zxc",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestBackendTpl(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				goatcounter.Memstore.Append([]goatcounter.Hit{
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/zxc", CreatedAt: now},
				}...)
				_, err := goatcounter.Memstore.Persist(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:   newBackend,
			path:     "/purge?path=/asd",
			auth:     true,
			wantCode: 200,
			wantBody: "<tr><td>2</td><td>/asd</td></tr>",
		},

		{
			setup: func(ctx context.Context) {
				one := int64(1)
				ss := goatcounter.Site{
					Name:   "Subsite",
					Code:   "subsite",
					Parent: &one,
					Plan:   goatcounter.PlanChild,
				}
				err := ss.Insert(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:   newBackend,
			path:     "/remove/2",
			auth:     true,
			wantCode: 200,
			wantBody: "Are you sure you want to remove the site Subsite",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestBackendPurge(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				goatcounter.Memstore.Append([]goatcounter.Hit{
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/zxc", CreatedAt: now},
				}...)
				_, err := goatcounter.Memstore.Persist(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:       newBackend,
			path:         "/purge",
			body:         map[string]string{"path": "/asd"},
			method:       "POST",
			auth:         true,
			wantFormCode: 303,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			var hits goatcounter.Hits
			err := hits.List(r.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(hits) != 1 {
				t.Fatalf("len is %d:\n%#v", len(hits), hits)
			}
		})
	}
}

func BenchmarkCount(b *testing.B) {
	ctx, clean := gctest.DB(b)
	defer clean()

	r, rr := newTest(ctx, "GET", "/count", nil)
	r.URL.RawQuery = url.Values{
		"p": {"/test.html"},
		"t": {"Benchmark test for /count"},
		"r": {"https://example.com/foo"},
	}.Encode()
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:72.0) Gecko/20100101 Firefox/72.0")
	r.Header.Set("Referer", "https://example.com/foo")

	handler := newBackend(zdb.MustGet(ctx)).ServeHTTP

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler(rr, r)
	}
}

func TestBarChart(t *testing.T) {
	id := tz.MustNew("", "Asia/Makassar").Loc()
	hi := tz.MustNew("", "Pacific/Honolulu").Loc()

	type testcase struct {
		zone                  string
		now, hit              time.Time
		wantHourly, wantDaily string
	}

	// The requested time is always from 2019-06-17 to 2019-06-18, in the local
	// TZ.
	tests := []testcase{
		{
			zone: "UTC",
			now:  date("2019-06-18 14:43", time.UTC),
			hit:  date("2019-06-18 12:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 0 views"></div>
				<div title="2019-06-18, 1 views"><div style="height:10%"></div></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-18 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-18 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-18 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-18 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-18 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-18 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-18 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-18 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-18 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-18 12:00 – 12:59, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-18 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-18 14:00 – 14:59, 0 views"></div>`, // Future not displayed
		},

		// +8
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 14:42", time.UTC),
			hit:  date("2019-06-18 12:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 0 views"></div>
				<div title="2019-06-18, 1 views"><div style="height:10%"></div></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-18 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-18 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-18 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-18 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-18 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-18 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-18 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-18 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-18 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-18 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-18 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-18 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-18 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-18 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-18 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-18 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-18 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-18 20:00 – 20:59, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-18 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-18 22:00 – 22:59, 0 views"></div>`,
		},

		// in the future, so nothing displayed
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 14:42", time.UTC),
			hit:  date("2019-06-18 23:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 0 views"></div>
				<div title="2019-06-18, 0 views"></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-18 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-18 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-18 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-18 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-18 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-18 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-18 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-18 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-18 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-18 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-18 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-18 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-18 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-18 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-18 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-18 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-18 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-18 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-18 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-18 22:00 – 22:59, 0 views"></div>`,
		},

		// The hit is added on the 17th, but displayed on the 18th
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 2:16", id),
			hit:  date("2019-06-17 18:15", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 0 views"></div>
				<div title="2019-06-18, 1 views"><div style="height:10%"></div></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 1 views"><div style="height:10%"></div></div>`,
		},

		// The hit is added on the 16th, but displayed on the 17th
		{
			zone: "Asia/Makassar",
			now:  date("2019-06-18 2:16", id),
			hit:  date("2019-06-16 18:15", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-18, 0 views"></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 0 views"></div>`,
		},

		// -10
		{
			zone: "Pacific/Honolulu",
			now:  date("2019-06-18 14:42", time.UTC),
			hit:  date("2019-06-18 12:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 0 views"></div>
				<div title="2019-06-18, 1 views"><div style="height:10%"></div></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 0 views"></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-18 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-18 4:00 – 4:59, 0 views"></div>`,
		},

		// The hit is added on the 18th, but displayed on the 17th
		{
			zone: "Pacific/Honolulu",
			now:  date("2019-06-18 14:42", hi),
			hit:  date("2019-06-18 2:42", time.UTC),
			wantDaily: `
				<div title="2019-06-17, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-18, 0 views"></div>`,
			wantHourly: `
				<div title="2019-06-17 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-17 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-17 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-17 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-17 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-17 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-17 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-17 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-17 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-17 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-17 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-17 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-17 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-17 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-17 14:00 – 14:59, 0 views"></div>
				<div title="2019-06-17 15:00 – 15:59, 0 views"></div>
				<div title="2019-06-17 16:00 – 16:59, 1 views"><div style="height:10%"></div></div>
				<div title="2019-06-17 17:00 – 17:59, 0 views"></div>
				<div title="2019-06-17 18:00 – 18:59, 0 views"></div>
				<div title="2019-06-17 19:00 – 19:59, 0 views"></div>
				<div title="2019-06-17 20:00 – 20:59, 0 views"></div>
				<div title="2019-06-17 21:00 – 21:59, 0 views"></div>
				<div title="2019-06-17 22:00 – 22:59, 0 views"></div>
				<div title="2019-06-17 23:00 – 23:59, 0 views"></div>
				<div title="2019-06-18 0:00 – 0:59, 0 views"></div>
				<div title="2019-06-18 1:00 – 1:59, 0 views"></div>
				<div title="2019-06-18 2:00 – 2:59, 0 views"></div>
				<div title="2019-06-18 3:00 – 3:59, 0 views"></div>
				<div title="2019-06-18 4:00 – 4:59, 0 views"></div>
				<div title="2019-06-18 5:00 – 5:59, 0 views"></div>
				<div title="2019-06-18 6:00 – 6:59, 0 views"></div>
				<div title="2019-06-18 7:00 – 7:59, 0 views"></div>
				<div title="2019-06-18 8:00 – 8:59, 0 views"></div>
				<div title="2019-06-18 9:00 – 9:59, 0 views"></div>
				<div title="2019-06-18 10:00 – 10:59, 0 views"></div>
				<div title="2019-06-18 11:00 – 11:59, 0 views"></div>
				<div title="2019-06-18 12:00 – 12:59, 0 views"></div>
				<div title="2019-06-18 13:00 – 13:59, 0 views"></div>
				<div title="2019-06-18 14:00 – 14:59, 0 views"></div>`,
		},
	}

	zlog.Config.Debug = []string{}

	run := func(t *testing.T, tt testcase, url, want string) {
		ctx, clean := gctest.DB(t)
		defer clean()

		ctx, site := gctest.Site(ctx, t, goatcounter.Site{
			CreatedAt: time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC),
			Settings:  goatcounter.SiteSettings{Timezone: tz.MustNew("", tt.zone)},
		})
		gctest.StoreHits(ctx, t, goatcounter.Hit{
			Site:      site.ID,
			CreatedAt: tt.hit.UTC(),
			Path:      "/a",
		})

		r, rr := newTest(ctx, "GET", url, nil)
		r.Host = site.Code + "." + cfg.Domain
		login(t, rr, r, site.ID)

		newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)

		doc, err := goquery.NewDocumentFromReader(rr.Body)
		if err != nil {
			t.Fatal(err)
		}
		c := doc.Find(".chart.chart-bar")
		if c.Length() != 1 {
			t.Fatalf("c.Length: %d", c.Length())
		}
		out, err := c.Html()
		if err != nil {
			t.Fatal(err)
		}

		out = strings.ReplaceAll(out, "</div>", "</div>\n")
		out = strings.ReplaceAll(out, "</div>\n</div>", "</div></div>")
		out = strings.TrimSpace(regexp.MustCompile(`[ \t]+<`).ReplaceAllString(out, "<"))

		want = `<span class="top max" title="Y-axis scale">10</span>` + "\n" +
			`<span class="half"></span>` + "\n" +
			strings.TrimSpace(strings.ReplaceAll(want, "\t", ""))

		if d := ztest.Diff(out, want); d != "" {
			t.Error(d)
			if sliceutil.InStringSlice(os.Args, "-test.v=true") {
				fmt.Println("Out:\n" + out)
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			goatcounter.Now = func() time.Time { return tt.now.UTC() }
			//t.Run("hourly", func(t *testing.T) {
			//	run(t, tt, "/?period-start=2019-06-17&period-end=2019-06-18", tt.wantHourly)
			//})
			t.Run("daily", func(t *testing.T) {
				run(t, tt, "/?period-start=2019-06-17&period-end=2019-06-18&daily=true", tt.wantDaily)
			})
		})
	}
}

func date(s string, tz *time.Location) time.Time {
	d, err := time.ParseInLocation("2006-01-02 15:04", s, tz)
	if err != nil {
		panic(err)
	}
	return d
}

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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/gctest"
	"zgo.at/isbot"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
	"zgo.at/ztest"
)

func TestBackendCount(t *testing.T) {
	goatcounter.Now = func() time.Time { return time.Date(2019, 6, 18, 14, 42, 0, 0, time.UTC) }

	tests := []struct {
		name     string
		query    url.Values
		set      func(r *http.Request)
		wantCode int
		hit      goatcounter.Hit
	}{
		{"no path", url.Values{}, nil, 400, goatcounter.Hit{}},
		{"invalid size", url.Values{"p": {"/x"}, "s": {"xxx"}}, nil, 400, goatcounter.Hit{}},

		{"", url.Values{"p": {"/foo.html"}}, nil, 200, goatcounter.Hit{
			Path: "/foo.html",
		}},

		{"add slash", url.Values{"p": {"foo.html"}}, nil, 200, goatcounter.Hit{
			Path: "/foo.html",
		}},

		{"event", url.Values{"p": {"foo.html"}, "e": {"true"}}, nil, 200, goatcounter.Hit{
			Path:  "foo.html",
			Event: true,
		}},

		{"params", url.Values{"p": {"/foo.html?a=b&c=d"}}, nil, 200, goatcounter.Hit{
			Path: "/foo.html?a=b&c=d",
		}},

		{"ref", url.Values{"p": {"/foo.html"}, "r": {"https://example.com"}}, nil, 200, goatcounter.Hit{
			Path:      "/foo.html",
			Ref:       "example.com",
			RefScheme: ztest.SP("h"),
		}},

		{"str ref", url.Values{"p": {"/foo.html"}, "r": {"example"}}, nil, 200, goatcounter.Hit{
			Path:      "/foo.html",
			Ref:       "example",
			RefScheme: ztest.SP("o"),
		}},

		{"ref params", url.Values{"p": {"/foo.html"}, "r": {"https://example.com?p=x"}}, nil, 200, goatcounter.Hit{
			Path:      "/foo.html",
			Ref:       "example.com",
			RefScheme: ztest.SP("h"),
		}},

		{"full", url.Values{"p": {"/foo.html"}, "t": {"XX"}, "r": {"https://example.com?p=x"}, "s": {"40,50,1"}}, nil, 200, goatcounter.Hit{
			Path:      "/foo.html",
			Title:     "XX",
			Ref:       "example.com",
			RefScheme: ztest.SP("h"),
			Size:      zdb.Floats{40, 50, 1},
		}},

		{"campaign", url.Values{"p": {"/foo.html"}, "q": {"ref=XXX"}}, nil, 200, goatcounter.Hit{
			Path:      "/foo.html",
			Ref:       "XXX",
			RefScheme: ztest.SP("c"),
		}},
		{"campaign_override", url.Values{"p": {"/foo.html?ref=AAA"}, "q": {"ref=XXX"}}, nil, 200, goatcounter.Hit{
			Path:      "/foo.html",
			Ref:       "XXX",
			RefScheme: ztest.SP("c"),
		}},

		{"bot", url.Values{"p": {"/a"}, "b": {"150"}}, nil, 200, goatcounter.Hit{
			Path: "/a",
			Bot:  150,
		}},
		{"googlebot", url.Values{"p": {"/a"}, "b": {"150"}}, func(r *http.Request) {
			r.Header.Set("User-Agent", "GoogleBot/1.0")
		}, 200, goatcounter.Hit{
			Path:    "/a",
			Bot:     int(isbot.BotShort),
			Browser: "GoogleBot/1.0",
		}},

		{"bot", url.Values{"p": {"/a"}, "b": {"100"}}, nil, 400, goatcounter.Hit{}},

		{"post", url.Values{"p": {"/foo.html"}}, func(r *http.Request) {
			r.Method = "POST"
		}, 200, goatcounter.Hit{
			Path: "/foo.html",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()

			ctx, site := gctest.Site(ctx, t, goatcounter.Site{
				CreatedAt: time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC),
			})

			r, rr := newTest(ctx, "GET", "/count?"+tt.query.Encode(), nil)
			r.Host = site.Code + "." + cfg.Domain
			if tt.set != nil {
				tt.set(r)
			}
			login(t, rr, r, site.ID)

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			if h := rr.Header().Get("X-Goatcounter"); h != "" {
				t.Logf("X-Goatcounter: %s", h)
			}
			ztest.Code(t, rr, tt.wantCode)

			if tt.wantCode >= 400 {
				return
			}

			_, err := goatcounter.Memstore.Persist(ctx)
			if err != nil {
				t.Fatal(err)
			}

			var hits []goatcounter.Hit
			err = zdb.MustGet(ctx).SelectContext(ctx, &hits, `select * from hits`)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) != 1 {
				t.Fatalf("len(hits) = %d: %#v", len(hits), hits)
			}

			h := hits[0]
			err = h.Validate(ctx)
			if err != nil {
				t.Errorf("Validate failed after get: %s", err)
			}

			one := int64(1)
			tt.hit.ID = h.ID
			tt.hit.Site = h.Site
			tt.hit.CreatedAt = goatcounter.Now()
			tt.hit.Session = &one // Should all be the same session.
			if tt.hit.Browser == "" {
				tt.hit.Browser = "GoatCounter test runner/1.0"
			}
			h.CreatedAt = h.CreatedAt.In(time.UTC)
			if d := ztest.Diff(h.String(), tt.hit.String()); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestBackendCountSessions(t *testing.T) {
	now := time.Date(2019, 6, 18, 14, 42, 0, 0, time.UTC)
	goatcounter.Now = func() time.Time { return now }
	ctx, clean := gctest.DB(t)
	defer clean()

	ctx1, _ := gctest.Site(ctx, t, goatcounter.Site{
		CreatedAt: time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC),
	})
	ctx2, _ := gctest.Site(ctx, t, goatcounter.Site{
		CreatedAt: time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC),
	})

	send := func(ctx context.Context, ua string) {
		site := goatcounter.MustGetSite(ctx)
		query := url.Values{"p": {"/" + zhttp.Secret()}}

		r, rr := newTest(ctx, "GET", "/count?"+query.Encode(), nil)
		r.Host = site.Code + "." + cfg.Domain
		r.Header.Set("User-Agent", ua)
		newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
		if h := rr.Header().Get("X-Goatcounter"); h != "" {
			t.Logf("X-Goatcounter: %s", h)
		}
		ztest.Code(t, rr, 200)

		_, err := goatcounter.Memstore.Persist(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	checkHits := func(ctx context.Context, n int) []goatcounter.Hit {
		var hits goatcounter.Hits
		_, err := hits.List(ctx, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != n {
			t.Errorf("len(hits) = %d; wanted %d", len(hits), n)
			for _, h := range hits {
				t.Logf("ID: %d; Site: %d; Session: %d\n", h.ID, h.Site, h.Session)
			}
			t.Fatal()
		}

		for _, h := range hits {
			err := h.Validate(ctx)
			if err != nil {
				t.Errorf("Validate failed after get: %s", err)
			}
		}
		return hits
	}

	checkSess := func(hits goatcounter.Hits, want []int) {
		var got []int
		for _, h := range hits {
			got = append(got, int(*h.Session))
			if !h.FirstVisit {
				t.Errorf("FirstVisit is false for %v", h)
			}
		}

		// TODO: test in order.
		sort.Ints(want)
		sort.Ints(got)

		w := fmt.Sprintf("%#v", want)
		g := fmt.Sprintf("%#v", got)
		if w != g {
			t.Errorf("wrong session\nwant: %s\ngot:  %s", w, g)
		}
	}

	rotate := func(ctx context.Context) {
		now = now.Add(12 * time.Hour)
		oldCur, _ := goatcounter.Salts.Get(ctx)

		err := goatcounter.Salts.Refresh(ctx)
		if err != nil {
			t.Fatal(err)
		}

		_, prev := goatcounter.Salts.Get(ctx)
		if prev != oldCur {
			t.Fatalf("salts not cycled?\noldCur: %s\nprev:   %s\n", oldCur, prev)
		}
	}

	// Ensure salts aren't cycled before they should.
	goatcounter.Salts.Get(ctx1)
	before := zdb.DumpString(ctx1, "select * from session_salts order by previous")
	now = now.Add(1 * time.Hour)
	goatcounter.Salts.Refresh(ctx1)
	after := zdb.DumpString(ctx1, "select * from session_salts order by previous")
	if d := ztest.Diff(before, after); d != "" {
		t.Fatalf("salts cycled too soon\n%s", d)
	}

	send(ctx1, "test")
	send(ctx1, "test")
	send(ctx1, "other")
	send(ctx2, "test")
	send(ctx2, "test")
	send(ctx1, "test")
	send(ctx1, "other")

	hits1 := checkHits(ctx1, 5)
	hits2 := checkHits(ctx2, 2)

	want := []int{1, 1, 2, 3, 3, 1, 2}
	checkSess(append(hits1, hits2...), want)

	// Rotate, should still use the same sessions.
	rotate(ctx1)
	send(ctx1, "test")
	send(ctx2, "test")
	hits1 = checkHits(ctx1, 6)
	hits2 = checkHits(ctx2, 3)
	want = []int{1, 1, 2, 3, 3, 1, 2, 1, 3}
	checkSess(append(hits1, hits2...), want)

	// Rotate again, should use new sessions from now on.
	rotate(ctx1)
	send(ctx1, "test")
	send(ctx2, "test")
	hits1 = checkHits(ctx1, 7)
	hits2 = checkHits(ctx2, 4)
	want = []int{1, 1, 2, 3, 3, 1, 2, 1, 3, 4, 5}
	checkSess(append(hits1, hits2...), want)
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
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestBackendTpl(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context, t *testing.T) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				gctest.StoreHits(ctx, t, []goatcounter.Hit{
					{Site: 1, Path: "/asd", Title: "AAA", CreatedAt: now},
					{Site: 1, Path: "/asd", Title: "AAA", CreatedAt: now},
					{Site: 1, Path: "/zxc", Title: "BBB", CreatedAt: now},
				}...)
			},
			router:   newBackend,
			path:     "/purge?path=/asd",
			auth:     true,
			wantCode: 200,
			wantBody: "<tr><td>2</td><td>/asd</td><td>AAA</td></tr>",
		},

		{
			setup: func(ctx context.Context, t *testing.T) {
				one := int64(1)
				ss := goatcounter.Site{
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
			wantBody: "Are you sure you want to remove the site",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestBackendPurge(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context, t *testing.T) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				gctest.StoreHits(ctx, t, []goatcounter.Hit{
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/zxc", CreatedAt: now},
				}...)
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
			bgrun.Wait()

			var hits goatcounter.Hits
			_, err := hits.List(r.Context(), 0, 0)
			if err != nil {
				t.Fatal(err)
			}

			if len(hits) != 1 {
				t.Logf("still have %d hits in DB (expected 1):\n", len(hits))
				for _, h := range hits {
					t.Logf("   ID: %d; Path: %q; Title: %q\n", h.ID, h.Path, h.Title)
				}
				t.FailNow()
			}
		})
	}
}

func TestBackendBarChart(t *testing.T) {
	id := tz.MustNew("", "Asia/Makassar").Loc()
	hi := tz.MustNew("", "Pacific/Honolulu").Loc()

	type testcase struct {
		zone                  string
		now, hit              time.Time
		wantHourly, wantDaily string
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

		// in the future, so nothing displayed
		{
			zone:        "Asia/Makassar",
			now:         date("2019-06-18 14:42", time.UTC),
			hit:         date("2019-06-18 23:42", time.UTC),
			wantNothing: true,
		},

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

	zlog.Config.Debug = []string{}

	run := func(t *testing.T, tt testcase, url, want string) {
		ctx, clean := gctest.DB(t)
		defer clean()

		ctx, site := gctest.Site(ctx, t, goatcounter.Site{
			CreatedAt: time.Date(2019, 01, 01, 0, 0, 0, 0, time.UTC),
			Settings:  goatcounter.SiteSettings{Timezone: tz.MustNew("", tt.zone)},
		})
		one := int64(1)
		gctest.StoreHits(ctx, t, goatcounter.Hit{
			Site:      site.ID,
			Session:   &one,
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
		if tt.wantNothing {
			// TODO: test this
			return
		}

		c := doc.Find(".chart.chart-bar")
		if c.Length() != 2 {
			t.Fatalf("c.Length: %d", c.Length())
		}
		totals, err := c.Eq(0).Html()
		if err != nil {
			t.Fatal(err)
		}
		out, err := c.Eq(1).Html()
		if err != nil {
			t.Fatal(err)
		}

		_ = totals // TODO: test too, although should probs be a different test.

		out = strings.ReplaceAll(out, "</div>", "</div>\n")
		out = strings.ReplaceAll(out, "</div>\n</div>", "</div></div>")
		out = strings.TrimSpace(regexp.MustCompile(`[ \t]+<`).ReplaceAllString(out, "<"))

		want = `<small class="bounce">` + "\n" +
			`<span title="Bounce rate">0%</span>` + "\n" +
			`</small>` + "\n" +
			`<span class="half"></span>` + "\n" +
			strings.TrimSpace(strings.ReplaceAll(want, "\t", ""))

		if d := ztest.Diff(out, want); d != "" {
			t.Error(d)
			if zstring.Contains(os.Args, "-test.v=true") {
				fmt.Println("Out:\n" + out)
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			goatcounter.Now = func() time.Time { return tt.now.UTC() }
			t.Run("hourly", func(t *testing.T) {
				run(t, tt, "/?period-start=2019-06-17&period-end=2019-06-18", tt.wantHourly)
			})
			t.Run("daily", func(t *testing.T) {
				run(t, tt, "/?period-start=2019-06-17&period-end=2019-06-18&daily=true", tt.wantDaily)
			})
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

func date(s string, tz *time.Location) time.Time {
	d, err := time.ParseInLocation("2006-01-02 15:04", s, tz)
	if err != nil {
		panic(err)
	}
	return d
}

func newBackend(db zdb.DB) chi.Router {
	return NewBackend(db, nil)
}

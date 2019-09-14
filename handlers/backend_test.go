// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
)

func TestBackendCount(t *testing.T) {
	tests := []handlerTest{
		{
			name:         "basic",
			router:       NewBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html"},
			wantCode:     200,
			wantFormCode: 200,
		},
		{
			name:         "params",
			router:       NewBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html?param=xxx"},
			wantCode:     200,
			wantFormCode: 200,
		},

		{
			name:         "ref",
			router:       NewBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com"},
			wantCode:     200,
			wantFormCode: 200,
		},
		{
			name:         "ref_params",
			router:       NewBackend,
			path:         "/count",
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com?p=xxx"},
			wantCode:     200,
			wantFormCode: 200,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			err := goatcounter.Memstore.Persist(r.Context())
			if err != nil {
				t.Fatal(err)
			}

			var hits []goatcounter.Hit
			err = goatcounter.MustGetDB(r.Context()).SelectContext(r.Context(), &hits, `select * from hits`)
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

func TestBackendAdmin(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context) {
				site := goatcounter.Site{Name: "new site", Code: "newsite", Plan: "p"}
				err := site.Insert(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:   NewBackend,
			path:     "/admin",
			auth:     true,
			wantCode: 200,
			wantBody: "<table>",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			b := rr.Body.String()
			if !strings.Contains(b, "<td>example.com</td>") {
				t.Errorf("no example.com")
			}
			if !strings.Contains(b, "<td>new site</td>") {
				t.Errorf("no new site")
			}
		})
	}
}

func TestBackendIndex(t *testing.T) {
	tests := []handlerTest{
		{
			name:     "no-data",
			router:   NewBackend,
			auth:     true,
			wantCode: 200,
			wantBody: "<strong>No data received</strong>",
		},

		{
			name: "basic",
			setup: func(ctx context.Context) {
				h := goatcounter.Hit{Path: "/asdfghjkl", Site: 1}
				err := h.Insert(ctx)
				if err != nil {
					panic(err)
				}
				db := goatcounter.MustGetDB(ctx).(*sqlx.DB)
				cron.Run(db)
			},
			router:   NewBackend,
			auth:     true,
			wantCode: 200,
			wantBody: "<h2>Pages <sup>(total 1 hits)</sup></h2>",
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
				hits := []goatcounter.Hit{
					{Path: "/asd", CreatedAt: now},
					{Path: "/asd", CreatedAt: now},
					{Path: "/zxc", CreatedAt: now},
				}
				for _, h := range hits {
					h.Site = 1
					err := h.Insert(ctx)
					if err != nil {
						panic(err)
					}
				}

			},
			router:   NewBackend,
			path:     "/export/hits.csv",
			auth:     true,
			wantCode: 200,
			wantBody: "/zxc",
		},

		{
			setup: func(ctx context.Context) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				browsers := []goatcounter.Browser{
					{Browser: "Firefox/68.0", CreatedAt: now},
					{Browser: "Chrome/77.0.123.666", CreatedAt: now},
					{Browser: "Firefox/69.0", CreatedAt: now},
				}
				for _, b := range browsers {
					b.Site = 1
					err := b.Insert(ctx)
					if err != nil {
						t.Fatal(err)
					}
				}

			},
			router:   NewBackend,
			path:     "/export/browsers.csv",
			auth:     true,
			wantCode: 200,
			wantBody: "Firefox",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

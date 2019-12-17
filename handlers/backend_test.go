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
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
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

func TestBackendAdmin(t *testing.T) {
	t.Skip() // TODO: Doesn't work with SQLite

	tests := []handlerTest{
		{
			setup: func(ctx context.Context) {
				site := goatcounter.Site{Name: "new site", Code: "newsite", Plan: goatcounter.PlanPersonal}
				err := site.Insert(ctx)
				if err != nil {
					panic(err)
				}

				user := goatcounter.User{Site: site.ID, Name: "Martin", Email: "martin@arp242.net"}
				err = user.Insert(context.WithValue(ctx, ctxkey.Site, site))
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
				goatcounter.Memstore.Append(goatcounter.Hit{Path: "/asdfghjkl", Site: 1})
				_, err := goatcounter.Memstore.Persist(ctx)
				if err != nil {
					panic(err)
				}
				db := zdb.MustGet(ctx).(*sqlx.DB)
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
			router:   NewBackend,
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
			router:   NewBackend,
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
			router:   NewBackend,
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
			router:       NewBackend,
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

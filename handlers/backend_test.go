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

	"zgo.at/goatcounter"
)

func TestCount(t *testing.T) {
	tests := []handlerTest{
		{
			name:         "basic",
			method:       "POST",
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html"},
			wantErr:      "",
			wantCode:     200,
			wantFormCode: 200,
			wantBody:     "",
			wantFormBody: "",
		},
		{
			name:         "params",
			method:       "POST",
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html?param=xxx"},
			wantErr:      "",
			wantCode:     200,
			wantFormCode: 200,
			wantBody:     "",
			wantFormBody: "",
		},

		{
			name:         "ref",
			method:       "POST",
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com"},
			wantErr:      "",
			wantCode:     200,
			wantFormCode: 200,
			wantBody:     "",
			wantFormBody: "",
		},
		{
			name:         "ref_params",
			method:       "POST",
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com?p=xxx"},
			wantErr:      "",
			wantCode:     200,
			wantFormCode: 200,
			wantBody:     "",
			wantFormBody: "",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			if tt.wantErr != "" {
				return
			}

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

func TestAdmin(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context) {
				site := goatcounter.Site{Name: "new site", Code: "newsite", Plan: "p"}
				err := site.Insert(ctx)
				if err != nil {
					panic(err)
				}
			},
			handler:  Backend{}.admin,
			path:     "/admin",
			body:     nil,
			wantErr:  "",
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

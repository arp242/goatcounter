// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
)

func TestHit(t *testing.T) {
	tests := []handlerTest{
		{
			name:         "basic",
			setup:        nil,
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html"},
			wantErr:      "",
			wantJSONCode: 200,
			wantFormCode: 200,
			wantJSONBody: "",
			wantFormBody: "",
		},
		{
			name:         "params",
			setup:        nil,
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html?param=xxx"},
			wantErr:      "",
			wantJSONCode: 200,
			wantFormCode: 200,
			wantJSONBody: "",
			wantFormBody: "",
		},

		{
			name:         "ref",
			setup:        nil,
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com"},
			wantErr:      "",
			wantJSONCode: 200,
			wantFormCode: 200,
			wantJSONBody: "",
			wantFormBody: "",
		},
		{
			name:         "ref_params",
			setup:        nil,
			handler:      Backend{}.count,
			body:         &goatcounter.Hit{Path: "/foo.html", Ref: "https://example.com?p=xxx"},
			wantErr:      "",
			wantJSONCode: 200,
			wantFormCode: 200,
			wantJSONBody: "",
			wantFormBody: "",
		},
	}

	zhttp.TplPath = "../tpl"
	zhttp.InitTpl(nil)
	zlog.Config.Outputs = []zlog.OutputFunc{} // Don't care about logs; don't spam.

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

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/zhttp"
	"zgo.at/zlog"
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
	zlog.Config.Output = func(zlog.Log) {} // Don't care about logs; don't spam.

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

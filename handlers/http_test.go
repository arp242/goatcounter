// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/jmoiron/sqlx"
	"github.com/teamwork/test"
	"github.com/teamwork/utils/jsonutil"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
)

type handlerTest struct {
	name         string
	setup        func(context.Context)
	router       func(*sqlx.DB) chi.Router
	path         string
	method       string
	auth         bool
	body         interface{}
	wantCode     int
	wantFormCode int
	wantBody     string
	wantFormBody string
}

func init() {
	zhttp.TplPath = "../tpl"
	zhttp.InitTpl(nil)
	zlog.Config.Outputs = []zlog.OutputFunc{} // Don't care about logs; don't spam.
}

func runTest(
	t *testing.T,
	tt handlerTest,
	fun func(*testing.T, *httptest.ResponseRecorder, *http.Request),
) {

	if tt.method == "" {
		tt.method = "GET"
	}
	if tt.path == "" {
		tt.path = "/"
	}

	t.Run(tt.name, func(t *testing.T) {
		sn := "json"
		if tt.method == "GET" {
			sn = "html"
		}

		t.Run(sn, func(t *testing.T) {
			ctx, clean := goatcounter.StartTest(t)
			defer clean()

			r, rr := newTest(ctx, tt.method, tt.path, bytes.NewReader(jsonutil.MustMarshal(tt.body)))
			if tt.setup != nil {
				tt.setup(ctx)
			}
			if tt.auth {
				login(t, rr, r)
			}

			tt.router(goatcounter.MustGetDB(ctx).(*sqlx.DB)).ServeHTTP(rr, r)
			test.Code(t, rr, tt.wantCode)
			if !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Errorf("wrong body\nwant: %s\ngot:  %s", tt.wantBody, rr.Body.String())
			}

			if fun != nil {
				// Don't use request context as it'll get cancelled.
				fun(t, rr, r.WithContext(ctx))
			}
		})

		if tt.method == "GET" {
			return
		}

		t.Run("form", func(t *testing.T) {
			ctx, clean := goatcounter.StartTest(t)
			defer clean()

			form := formBody(tt.body)
			r, rr := newTest(ctx, tt.method, tt.path, strings.NewReader(form))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Content-Length", fmt.Sprintf("%d", len(form)))
			if tt.setup != nil {
				tt.setup(ctx)
			}
			if tt.auth {
				login(t, rr, r)
			}

			tt.router(goatcounter.MustGetDB(ctx).(*sqlx.DB)).ServeHTTP(rr, r)
			test.Code(t, rr, tt.wantFormCode)
			if !strings.Contains(rr.Body.String(), tt.wantFormBody) {
				t.Errorf("wrong body\nwant: %q\ngot:  %q", tt.wantFormBody, rr.Body.String())
			}

			if fun != nil {
				// Don't use request context as it'll get cancelled.
				fun(t, rr, r.WithContext(ctx))
			}
		})
	})
}

func login(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
	t.Helper()

	// Insert user
	u := goatcounter.User{Site: 1, Name: "Example", Email: "test@example.com"}
	err := u.Insert(r.Context())
	if err != nil {
		t.Fatal(err)
	}

	// Login user
	err = u.Login(r.Context())
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Cookie", "key="+*u.Key.LoginKey)
	*r = *r.WithContext(context.WithValue(r.Context(), ctxkey.User, u))
}

func newTest(ctx context.Context, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	return test.NewRequest(method, path, body).WithContext(ctx), httptest.NewRecorder()
}

// Convert anything to an "application/x-www-form-urlencoded" form.
//
// Use github.com/teamwork/test.Multipart for a multipart form.
//
// Note: this is primitive, but enough for now.
func formBody(i interface{}) string {
	var m map[string]string
	jsonutil.MustUnmarshal(jsonutil.MustMarshal(i), &m)

	f := make(url.Values)
	for k, v := range m {
		f[k] = []string{v}
	}

	// TODO: null values are:
	// email=foo%40example.com&frequency=
	return f.Encode()
}

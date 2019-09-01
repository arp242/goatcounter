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

	"github.com/teamwork/test"
	"github.com/teamwork/utils/jsonutil"
	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
)

type handlerTest struct {
	name         string
	setup        func(context.Context)
	handler      func(http.ResponseWriter, *http.Request) error
	path         string
	method       string
	body         interface{}
	wantErr      string
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
				tt.setup(r.Context())
			}

			zhttp.Wrap(tt.handler)(rr, r)
			test.Code(t, rr, tt.wantCode)

			if !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Errorf("wrong body\nwant: %s\ngot:  %s", tt.wantBody, rr.Body.String())
			}

			if fun != nil {
				fun(t, rr, r)
			}
		})

		if tt.method == "GET" {
			return
		}

		t.Run("form", func(t *testing.T) {
			ctx, clean := goatcounter.StartTest(t)
			defer clean()

			form := formBody(tt.body)
			r, rr := newTest(ctx, "POST", "/", strings.NewReader(form))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Content-Length", fmt.Sprintf("%d", len(form)))
			if tt.setup != nil {
				tt.setup(r.Context())
			}

			zhttp.Wrap(tt.handler)(rr, r)
			test.Code(t, rr, tt.wantFormCode)

			if !strings.Contains(rr.Body.String(), tt.wantFormBody) {
				t.Errorf("wrong body\nwant: %q\ngot:  %q", tt.wantFormBody, rr.Body.String())
			}

			if fun != nil {
				fun(t, rr, r)
			}
		})
	})
}

// TODO: use actual middleware.
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

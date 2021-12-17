// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zgo"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/ztest"
	"zgo.at/ztpl"
)

type handlerTest struct {
	name         string
	setup        func(context.Context, *testing.T)
	router       func(zdb.DB) chi.Router
	path         string
	method       string
	auth         bool
	body         interface{}
	wantCode     int
	wantFormCode int
	wantBody     string
	wantFormBody string
	want         string
	serve        bool
}

func init() {
	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter,
		blackmail.MailerOut(new(bytes.Buffer)))

	files, _ := fs.Sub(os.DirFS(zgo.ModuleRoot()), "tpl")
	err := ztpl.Init(files)
	if err != nil {
		panic(err)
	}

	ztest.DefaultHost = "test.example.com"
	if zruntime.TestVerbose() {
		zlog.Config.Debug = []string{"all"}
	} else {
		zlog.Config.Outputs = []zlog.OutputFunc{} // Don't care about logs; don't spam.
	}
}

func TestMain(m *testing.M) {
	os.Exit(ztpl.TestTemplateExecution(m,
		// Don't need tests.
		"", "bosmang.gohtml", "bosmang_site.gohtml", "bosmang_cache.gohtml",
		"i18n_list.gohtml", "i18n_show.gohtml",

		// Tested in tpl_test.go
		"email_export_done.gotxt", "email_forgot_site.gotxt", "email_import_done.gotxt",
		"email_import_error.gotxt", "email_password_reset.gotxt", "email_verify.gotxt",
		"email_adduser.gotxt",

		// TODO
		"_dashboard_pages_refs.gohtml",
		"_dashboard_pages_text.gohtml",
		"_dashboard_pages_text_rows.gohtml",
		"bosmang_sites.gohtml",
		"bosmang_bgrun.gohtml",
		"bosmang_metrics.gohtml",
		"settings_server.gohtml",

		// TODO: hard to test; requires a browser as the JS generates secret
		// Stripe stuff in JS.
		"billing.gohtml",
	))
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

		if tt.wantCode > 0 {
			t.Run(sn, func(t *testing.T) {
				ctx := gctest.DB(t)
				goatcounter.Config(ctx).GoatcounterCom = !tt.serve

				r, rr := newTest(ctx, tt.method, tt.path, bytes.NewReader(zjson.MustMarshal(tt.body)))
				if tt.setup != nil {
					tt.setup(ctx, t)
				}
				if tt.auth {
					login(t, r)
				}

				tt.router(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
				ztest.Code(t, rr, tt.wantCode)
				if !strings.Contains(rr.Body.String(), tt.wantBody) {
					t.Errorf("wrong body\nwant: %s\ngot:  %s", tt.wantBody, rr.Body.String())
				}

				if fun != nil {
					// Don't use request context as it'll get cancelled.
					fun(t, rr, r.WithContext(ctx))
				}
			})
		}

		if tt.method == "GET" {
			return
		}

		t.Run("form", func(t *testing.T) {
			ctx := gctest.DB(t)
			goatcounter.Config(ctx).GoatcounterCom = !tt.serve

			form := formBody(tt.body)
			r, rr := newTest(ctx, tt.method, tt.path, strings.NewReader(form))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Content-Length", fmt.Sprintf("%d", len(form)))
			if tt.setup != nil {
				tt.setup(ctx, t)
			}
			if tt.auth {
				login(t, r)
			}

			tt.router(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			if f := zhttp.ReadFlash(rr, r); f != nil {
				t.Logf("flash message (%s): %s", f.Level, f.Message)
			}
			ztest.Code(t, rr, tt.wantFormCode)
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

func login(t *testing.T, r *http.Request) {
	t.Helper()

	// Login user
	u := User(r.Context())
	err := u.Login(r.Context())
	if err != nil {
		t.Fatal(err)
	}

	if u.LoginToken == nil {
		t.Fatal("u.LoginToken is nil? Should never happen!")
	}

	// Set CSRF token.
	// TODO: only works for form requests, which is okay as zhttp csrf checking
	// only works for forms for now.
	err = r.ParseForm()
	if err != nil {
		t.Fatal(err)
	}
	r.Form.Set("csrf", *u.Token)

	r.Header.Set("Cookie", "key="+*u.LoginToken)
}

func newLoginTest(t *testing.T, ctx context.Context, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	r, rr := newTest(ctx, method, path, body)
	login(t, r)
	return r, rr
}

func newTest(ctx context.Context, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	site := Site(ctx)
	r, rr := ztest.NewRequest(method, path, body).WithContext(ctx), httptest.NewRecorder()
	r.Header.Set("User-Agent", "GoatCounter test runner/1.0")
	r.Host = site.Code + "." + goatcounter.Config(ctx).Domain
	return r, rr
}

// Convert anything to an "application/x-www-form-urlencoded" form.
//
// Use github.com/teamwork/test.Multipart for a multipart form.
//
// Note: this is primitive, but enough for now.
func formBody(i interface{}) string {
	var m map[string]string
	zjson.MustUnmarshal(zjson.MustMarshal(i), &m)

	f := make(url.Values)
	for k, v := range m {
		f[k] = []string{v}
	}

	// TODO: null values are:
	// email=foo%40example.com&frequency=
	return f.Encode()
}

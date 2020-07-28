// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zvalidate"
)

func jsonCmp(a, b string) bool {
	var aj, bj json.RawMessage
	err := json.Unmarshal([]byte(a), &aj)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal([]byte(b), &bj)
	if err != nil {
		panic(err)
	}

	aout, err := json.Marshal(aj)
	if err != nil {
		panic(err)
	}
	bout, err := json.Marshal(bj)
	if err != nil {
		panic(err)
	}

	return string(aout) == string(bout)
}

func newAPITest(ctx context.Context, t *testing.T,
	method, path string, body io.Reader,
	perm goatcounter.APITokenPermissions,
) (*http.Request, *httptest.ResponseRecorder) {

	token := goatcounter.APIToken{
		SiteID:      Site(ctx).ID,
		UserID:      goatcounter.GetUser(ctx).ID,
		Name:        "test",
		Permissions: perm,
	}
	err := token.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}

	r, rr := newTest(ctx, method, path, body)
	r.Header.Set("Authorization", "Bearer "+token.Token)
	return r, rr
}

func TestAPIBasics(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		t.Run("no-auth", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "GET", "/api/v0/test", nil, goatcounter.APITokenPermissions{})

			delete(r.Header, "Authorization")
			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"no Authorization header"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("wrong-auth", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "GET", "/api/v0/test", nil, goatcounter.APITokenPermissions{})

			r.Header.Set("Authorization", r.Header.Get("Authorization")+"x")
			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"unknown token"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("no-perm", func(t *testing.T) {
			body := bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
				"perm": goatcounter.APITokenPermissions{Export: true, Count: true},
			}))
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", body, goatcounter.APITokenPermissions{})

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"requires [count export] permissions"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("404", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/doesnt-exist", nil, goatcounter.APITokenPermissions{})

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 404)

			want := `{"error":"Not Found"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("500", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{"status":500}`),
				goatcounter.APITokenPermissions{})

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 500)

			want := `{"error":"unexpected error code ‘`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("invalid json", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{{{{`),
				goatcounter.APITokenPermissions{})

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"error":"invalid JSON:`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("panic", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{"panic":true}`),
				goatcounter.APITokenPermissions{})

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 500)

			want := `{"error":"unexpected error code ‘`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("ct", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", nil, goatcounter.APITokenPermissions{})

			r.Header.Set("Content-Type", "text/html")

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 415)

			want := `<!DOCTYPE html>`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("validate", func(t *testing.T) {
			v := zvalidate.New()
			v.Required("r", "")
			v.Email("e", "asd")

			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
					"validate": v,
				})),
				goatcounter.APITokenPermissions{})

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"errors":{"e":["must be a valid email address"],"r":["must be set"]}}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})
	})

	t.Run("no-perm", func(t *testing.T) {
		ctx, clean := gctest.DB(t)
		defer clean()

		r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", nil, goatcounter.APITokenPermissions{})

		newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
	})

	t.Run("check-perm", func(t *testing.T) {
		ctx, clean := gctest.DB(t)
		defer clean()

		body := bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
			"perm": goatcounter.APITokenPermissions{Export: true, Count: true},
		}))
		r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", body, goatcounter.APITokenPermissions{
			Export: true, Count: true,
		})

		newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
	})
}

func TestAPICount(t *testing.T) {
	tests := []struct {
		body     APICountRequest
		wantCode int
		wantRet  string
		want     string
	}{
		{APICountRequest{}, 400, `{"error":"no hits"}`, ``},

		// {
		// 	APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
		// 		{Path: "/", CreatedAt: goatcounter.Now().Add(5 * time.Minute)},
		// 	}}, 400, `{"errors":{"0":"created_at: in the future.\n"}}`, "",
		// },

		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo"},
				{Path: "/bar", CreatedAt: time.Date(2020, 1, 18, 14, 42, 0, 0, time.UTC)},
			}},
			202, respOK, `
			path  title  event  ua  uabot  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			/foo         0          7                       00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			/bar         0          7                       00112233445566778899aabbccddef01  0         NULL              1      2020-01-18 14:42:00
			`,
		},

		// Fill in most fields.
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo", Title: "A", Ref: "y", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", Location: "ET", Size: zdb.Floats{42, 666, 2}},
			}},
			202, respOK, `
			path  title  event  ua           uabot  browser    system  session                           bot  ref  ref_s  size      loc  first  created_at
			/foo  A      0      ~Z (~L) ~f1  1      Firefox 1  Linux   00112233445566778899aabbccddef01  0    y    o      42,666,2  ET   1      2020-06-18 14:42:00
			`,
		},

		// Event
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Event: zdb.Bool(true), Path: "/foo", Title: "A", Ref: "y", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", Location: "ET", Size: zdb.Floats{42, 666, 2}},
			}},
			202, respOK, `
			path  title  event  ua           uabot  browser    system  session                           bot  ref  ref_s  size      loc  first  created_at
			foo   A      1      ~Z (~L) ~f1  1      Firefox 1  Linux   00112233445566778899aabbccddef01  0    y    o      42,666,2  ET   1      2020-06-18 14:42:00
			`,
		},

		// Calculate session from IP+UserAgent
		{
			APICountRequest{Hits: []APICountRequestHit{
				{Path: "/foo", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", IP: "66.66.66.66"},
				{Path: "/foo", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", IP: "66.66.66.67"},
				{Path: "/foo", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", IP: "66.66.66.66"},
			}},
			202, respOK, `
			path  title  event  ua           uabot  browser    system  session                           bot  ref  ref_s  size  loc  first  created_at
			/foo         0      ~Z (~L) ~f1  1      Firefox 1  Linux   00112233445566778899aabbccddef01  0         NULL         US   1      2020-06-18 14:42:00
			/foo         0      ~Z (~L) ~f1  1      Firefox 1  Linux   00112233445566778899aabbccddef02  0         NULL         US   1      2020-06-18 14:42:00
			/foo         0      ~Z (~L) ~f1  1      Firefox 1  Linux   00112233445566778899aabbccddef01  0         NULL         US   0      2020-06-18 14:42:00
			`,
		},

		// UserSessionID
		{
			APICountRequest{Hits: []APICountRequestHit{
				{Path: "/foo", Session: "a"},
				{Path: "/foo", Session: "b"},
				{Path: "/foo", Session: "a"},
			}},
			202, respOK, `
			path  title  event  ua  uabot  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			/foo         0          7                       00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			/foo         0          7                       00112233445566778899aabbccddef02  0         NULL              1      2020-06-18 14:42:00
			/foo         0          7                       00112233445566778899aabbccddef01  0         NULL              0      2020-06-18 14:42:00
			`,
		},

		// Don't persist if session is blank.
		{
			APICountRequest{Hits: []APICountRequestHit{
				{Path: "/foo", Session: "a"},
				{Path: "/foo"},
			}},
			400, `{"errors":{"1":"session or browser/IP not set; use no_sessions if you don't want to track unique visits"}}`, `
			path  title  event  ua  uabot  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			/foo         0          7                       00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			`,
		},
	}

	defer gctest.SwapNow(t, "2020-06-18 14:42:00")()
	perm := goatcounter.APITokenPermissions{Count: true}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/count",
				bytes.NewReader(zjson.MustMarshal(tt.body)), perm)

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)
			if !jsonCmp(rr.Body.String(), tt.wantRet) {
				t.Errorf("\nout:  %s\nwant: %s", rr.Body.String(), tt.wantRet)
			}

			gctest.StoreHits(ctx, t, false)

			tt.want = strings.TrimSpace(strings.ReplaceAll(tt.want, "\t", ""))
			//got := strings.TrimSpace(zdb.DumpString(ctx, `select * from view_hits`, zdb.DumpVertical))
			got := strings.TrimSpace(zdb.DumpString(ctx, `select * from view_hits`))

			if strings.Count(got, "\n") == 0 { // No data, only the header.
				got = ""
			}

			if d := ztest.Diff(got, tt.want); d != "" {
				t.Errorf(d)
				//fmt.Println(got)
			}
		})
	}
}

func TestAPISitesUpdate(t *testing.T) {
	stdsite := goatcounter.Site{Code: "gctest", Plan: "personal"}
	stdsite.Defaults(context.Background())

	tests := []struct {
		method, body string
		wantCode     int
		want         func() goatcounter.Site
	}{
		{"POST", `{}`, 200, func() goatcounter.Site {
			s := stdsite
			s.Settings.Campaigns = zdb.Strings{} // Gets reset as it's not sent.
			return s
		}},

		{"PATCH", `{}`, 200, func() goatcounter.Site {
			s := stdsite
			return s
		}},
	}

	perm := goatcounter.APITokenPermissions{SiteCreate: true, SiteRead: true,
		SiteUpdate: true}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()

			site := Site(ctx)

			r, rr := newAPITest(ctx, t, tt.method, fmt.Sprintf("/api/v0/sites/%d", site.ID),
				strings.NewReader(tt.body), perm)

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			var retSite goatcounter.Site
			d := json.NewDecoder(rr.Body)
			d.AllowReadonlyFields()
			err := d.Decode(&retSite)
			if err != nil {
				t.Fatal(err)
			}

			w := tt.want()
			w.ID = retSite.ID
			retSite.CreatedAt = w.CreatedAt
			retSite.UpdatedAt = nil

			got := string(zjson.MustMarshalIndent(retSite, "", "  "))
			want := string(zjson.MustMarshalIndent(w, "", "  "))
			if d := ztest.Diff(got, want); d != "" {
				t.Error(d)
			}
		})
	}
}

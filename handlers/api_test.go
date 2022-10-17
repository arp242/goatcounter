// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

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

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
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
	perm zint.Bitflag64,
) (*http.Request, *httptest.ResponseRecorder) {

	token := goatcounter.APIToken{
		SiteID:      Site(ctx).ID,
		UserID:      User(ctx).ID,
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
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "GET", "/api/v0/test", nil, 0)

			delete(r.Header, "Authorization")
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"no Authorization header"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("wrong-auth", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "GET", "/api/v0/test", nil, 0)

			r.Header.Set("Authorization", r.Header.Get("Authorization")+"x")
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"unknown token"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("no-perm", func(t *testing.T) {
			body := bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
				"perm": goatcounter.APIPermExport | goatcounter.APIPermCount,
			}))
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", body, 0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"requires 'count', 'export' permissions"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("404", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/doesnt-exist", nil, 0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 404)

			want := `{"error":"not found"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("500", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{"status":500}`),
				0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 500)

			want := `{"error":"unexpected error code ‘`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("invalid json", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{{{{`),
				0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"error":"invalid JSON:`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("panic", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{"panic":true}`),
				0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 500)

			want := `{"error":"unexpected error code ‘`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("ct", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", nil, 0)

			r.Header.Set("Content-Type", "text/html")

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
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

			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
					"validate": v,
				})),
				0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"errors":{"e":["must be a valid email address"],"r":["must be set"]}}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})
	})

	t.Run("context", func(t *testing.T) {
		ctx := gctest.DB(t)
		r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
			bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
				"context": true,
			})),
			0)

		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
	})

	t.Run("no-perm", func(t *testing.T) {
		ctx := gctest.DB(t)
		r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", nil, 0)

		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
	})

	t.Run("check-perm", func(t *testing.T) {
		ctx := gctest.DB(t)

		body := bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
			"perm": goatcounter.APIPermExport | goatcounter.APIPermCount,
		}))
		r, rr := newAPITest(ctx, t, "POST", "/api/v0/test", body,
			goatcounter.APIPermExport|goatcounter.APIPermCount)

		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
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

		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo"},
				{Path: "/bar", CreatedAt: time.Date(2020, 1, 18, 14, 42, 0, 0, time.UTC)},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  ua  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                           00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			2       1        /bar         0                           00112233445566778899aabbccddef01  0         NULL              1      2020-01-18 14:42:00
			`,
		},

		// Fill in most fields.
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo", Title: "A", Ref: "y", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", Location: "ET", Size: goatcounter.Floats{42, 666, 2}},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  ua           browser    system  session                           bot  ref  ref_s  size      loc  first  created_at
			1       1        /foo  A      0      ~Z (~L) ~f1  Firefox 1  Linux   00112233445566778899aabbccddef01  0    y    o      42,666,2  ET   1      2020-06-18 14:42:00
			`,
		},

		// Event
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Event: zbool.Bool(true), Path: "/foo", Title: "A", Ref: "y", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", Location: "ET", Size: goatcounter.Floats{42, 666, 2}},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  ua           browser    system  session                           bot  ref  ref_s  size      loc  first  created_at
			1       1        foo   A      1      ~Z (~L) ~f1  Firefox 1  Linux   00112233445566778899aabbccddef01  0    y    o      42,666,2  ET   1      2020-06-18 14:42:00
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
			hit_id  site_id  path  title  event  ua           browser    system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0      ~Z (~L) ~f1  Firefox 1  Linux   00112233445566778899aabbccddef01  0         NULL         US   1      2020-06-18 14:42:00
			2       1        /foo         0      ~Z (~L) ~f1  Firefox 1  Linux   00112233445566778899aabbccddef02  0         NULL         US   1      2020-06-18 14:42:00
			3       1        /foo         0      ~Z (~L) ~f1  Firefox 1  Linux   00112233445566778899aabbccddef01  0         NULL         US   0      2020-06-18 14:42:00
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
			hit_id  site_id  path  title  event  ua  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                           00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			2       1        /foo         0                           00112233445566778899aabbccddef02  0         NULL              1      2020-06-18 14:42:00
			3       1        /foo         0                           00112233445566778899aabbccddef01  0         NULL              0      2020-06-18 14:42:00
			`,
		},

		// Don't persist if session is blank.
		{
			APICountRequest{Hits: []APICountRequestHit{
				{Path: "/foo", Session: "a"},
				{Path: "/foo"},
			}},
			400, `{"errors":{"1":"session or browser/IP not set; use no_sessions if you don't want to track unique visits"}}`, `
			hit_id  site_id  path  title  event  ua  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                           00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			`,
		},

		// Filter bots
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo"},
				{Path: "/foo", UserAgent: "curl/7.8"},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  ua        browser   system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                                  00112233445566778899aabbccddef01  0         NULL              1      2020-06-18 14:42:00
			2       1        /foo         0      curl/7.8  curl 7.8          00112233445566778899aabbccddef02  7         NULL              1      2020-06-18 14:42:00
			`,
		},

		// Filter IP
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo", IP: "1.1.1.1"},
			}},
			202, respOK, ``,
		},
		{
			APICountRequest{NoSessions: true, Filter: []string{"ip"}, Hits: []APICountRequestHit{
				{Path: "/foo", IP: "1.1.1.1"},
			}},
			202, respOK, ``,
		},
		{
			APICountRequest{NoSessions: true, Filter: []string{}, Hits: []APICountRequestHit{
				{Path: "/foo", IP: "1.2.3.4"},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  ua  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                           00112233445566778899aabbccddef01  0         NULL         AU   1      2020-06-18 14:42:00
			`,
		},
	}

	ztime.SetNow(t, "2020-06-18 14:42:00")
	perm := goatcounter.APIPermCount

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := gctest.DB(t)
			site := Site(ctx)
			site.Settings.IgnoreIPs = []string{"1.1.1.1"}
			err := site.Update(ctx)
			if err != nil {
				t.Fatal(err)
			}

			r, rr := newAPITest(ctx, t, "POST", "/api/v0/count",
				bytes.NewReader(zjson.MustMarshal(tt.body)), perm)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)
			if !jsonCmp(rr.Body.String(), tt.wantRet) {
				t.Errorf("\nout:  %s\nwant: %s", rr.Body.String(), tt.wantRet)
			}

			gctest.StoreHits(ctx, t, false)

			tt.want = strings.TrimSpace(strings.ReplaceAll(tt.want, "\t", ""))
			got := strings.TrimSpace(zdb.DumpString(ctx, `
				select
					hits.hit_id,
					hits.site_id,

					paths.path,
					paths.title,
					paths.event,

					user_agents.ua,
					browsers.name || ' ' || browsers.version as browser,
					systems.name  || ' ' || systems.version  as system,

					hits.session,
					hits.bot,
					hits.ref,
					hits.ref_scheme as ref_s,
					hits.size,
					hits.location as loc,
					hits.first_visit as first,
					hits.created_at
				from hits
				join paths       using (path_id)
				join user_agents using (user_agent_id)
				join browsers    using (browser_id)
				join systems     using (system_id)
				order by hit_id asc`))

			if strings.Count(got, "\n") == 0 { // No data, only the header.
				got = ""
			}

			if d := ztest.Diff(got, tt.want); d != "" {
				t.Errorf(d)
			}
		})
	}
}

func TestAPISitesCreate(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:13:14")
	now := ztime.Now()

	tests := []struct {
		serve    bool
		body     string
		wantCode int
		want     func(*goatcounter.Site)
	}{
		{false, `{"code":"apitest"}`, 200, func(s *goatcounter.Site) {
			s.Code = "apitest"
			s.Parent = ztype.Ptr(int64(1))
		}},
		{true, `{"cname":"apitest.localhost"}`, 200, func(s *goatcounter.Site) {
			s.Cname = ztype.Ptr("apitest.localhost")
			s.Parent = ztype.Ptr(int64(1))
			s.CnameSetupAt = &now
		}},

		// Ignore plan.
		{false, `{"code":"apitest"}`, 200, func(s *goatcounter.Site) {
			s.Code = "apitest"
			s.Parent = ztype.Ptr(int64(1))
		}},
		{true, `{"cname":"apitest.localhost"}`, 200, func(s *goatcounter.Site) {
			s.Cname = ztype.Ptr("apitest.localhost")
			s.Parent = ztype.Ptr(int64(1))
			s.CnameSetupAt = &now
		}},
	}

	perm := goatcounter.APIPermSiteCreate | goatcounter.APIPermSiteRead | goatcounter.APIPermSiteUpdate
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := gctest.DB(t)
			goatcounter.Config(ctx).GoatcounterCom = !tt.serve

			r, rr := newAPITest(ctx, t, "PUT", "/api/v0/sites",
				strings.NewReader(tt.body), perm)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			var retSite goatcounter.Site
			d := json.NewDecoder(rr.Body)
			d.AllowReadonlyFields()
			err := d.Decode(&retSite)
			if err != nil {
				t.Fatal(err)
			}

			var w goatcounter.Site
			w.Defaults(ctx)
			tt.want(&w)

			w.ID = retSite.ID
			if tt.serve {
				retSite.Code = w.Code
			}
			retSite.CreatedAt = w.CreatedAt
			retSite.FirstHitAt = w.FirstHitAt
			retSite.UpdatedAt = nil

			got := string(zjson.MustMarshalIndent(retSite, "", "  "))
			want := string(zjson.MustMarshalIndent(w, "", "  "))
			if d := ztest.Diff(got, want); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestAPISitesUpdate(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:13:14")
	now := ztime.Now()

	tests := []struct {
		serve        bool
		method, body string
		wantCode     int
		want         func(s *goatcounter.Site)
	}{
		{false, "PATCH", `{}`, 200, func(s *goatcounter.Site) {
			s.Code = "gctest"
			s.Cname = ztype.Ptr("gctest.localhost")
		}},
		{false, "POST", `{}`, 200, func(s *goatcounter.Site) {
			s.Code = "gctest"
			//s.Cname = ztype.Ptr("gctest.localhost")
		}},
	}

	_ = now

	perm := goatcounter.APIPermSiteCreate | goatcounter.APIPermSiteRead | goatcounter.APIPermSiteUpdate
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := gctest.DB(t)
			goatcounter.Config(ctx).GoatcounterCom = !tt.serve

			site := Site(ctx)

			r, rr := newAPITest(ctx, t, tt.method, fmt.Sprintf("/api/v0/sites/%d", site.ID),
				strings.NewReader(tt.body), perm)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			var retSite goatcounter.Site
			d := json.NewDecoder(rr.Body)
			d.AllowReadonlyFields()
			err := d.Decode(&retSite)
			if err != nil {
				t.Fatal(err)
			}

			var w goatcounter.Site
			w.Defaults(ctx)
			tt.want(&w)

			w.ID = retSite.ID
			retSite.CreatedAt = w.CreatedAt
			retSite.FirstHitAt = w.FirstHitAt
			retSite.UpdatedAt = nil

			got := string(zjson.MustMarshalIndent(retSite, "", "  "))
			want := string(zjson.MustMarshalIndent(w, "", "  "))
			if d := ztest.Diff(got, want); d != "" {
				t.Error(d)
			}
		})
	}
}

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
	"strconv"
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
	"zgo.at/zvalidate"
)

func newAPITest(ctx context.Context, t *testing.T,
	method, path string, body io.Reader,
	perm zint.Bitflag64,
) (*http.Request, *httptest.ResponseRecorder) {

	zhttp.DefaultDecoder = zhttp.NewDecoder(true, false)
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
			ztest.Code(t, rr, 401)

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
			ztest.Code(t, rr, 401)

			want := `{"error":"unknown token"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("no-perm", func(t *testing.T) {
			body := bytes.NewReader(zjson.MustMarshal(map[string]any{
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
				bytes.NewReader(zjson.MustMarshal(map[string]any{
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

		t.Run("unknown JSON", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
				strings.NewReader(`{"unknown":"aa"}`), 0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"error":"unknown parameter: \"unknown\""}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})
		t.Run("unknown query", func(t *testing.T) {
			ctx := gctest.DB(t)
			r, rr := newAPITest(ctx, t, "GET", "/api/v0/test?unknown=1", nil, 0)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"error":"unknown parameter: \"unknown\""}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})
	})

	t.Run("context", func(t *testing.T) {
		ctx := gctest.DB(t)
		r, rr := newAPITest(ctx, t, "POST", "/api/v0/test",
			bytes.NewReader(zjson.MustMarshal(map[string]any{
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

		body := bytes.NewReader(zjson.MustMarshal(map[string]any{
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
			hit_id  site_id  path  title  event  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                       00112233445566778899aabbccddef01  0         NULL   NULL       1      2020-06-18 14:42:00
			2       1        /bar         0                       00112233445566778899aabbccddef01  0         NULL   NULL       1      2020-01-18 14:42:00
			`,
		},

		// Fill in most fields.
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo", Title: "A", Ref: "y", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", Location: "ET", Size: goatcounter.Floats{42, 666, 2}},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  browser    system  session                           bot  ref  ref_s  size        loc  first  created_at
			1       1        /foo  A      0      Firefox 1  Linux   00112233445566778899aabbccddef01  0    y    o      42,666,2.0  ET   1      2020-06-18 14:42:00
			`,
		},

		// Event
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Event: zbool.Bool(true), Path: "/foo", Title: "A", Ref: "y", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", Location: "ET", Size: goatcounter.Floats{42, 666, 2}},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  browser    system  session                           bot  ref  ref_s  size        loc  first  created_at
			1       1        foo   A      1      Firefox 1  Linux   00112233445566778899aabbccddef01  0    y    o      42,666,2.0  ET   1      2020-06-18 14:42:00
			`,
		},

		// Calculate session from IP+UserAgent
		{
			APICountRequest{Hits: []APICountRequestHit{
				{Path: "/foo", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", IP: "51.171.91.33"},
				{Path: "/foo", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", IP: "66.66.66.67"},
				{Path: "/foo", UserAgent: "Mozilla/5.0 (Linux) Firefox/1", IP: "51.171.91.33"},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  browser    system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0      Firefox 1  Linux   00112233445566778899aabbccddef01  0         NULL   NULL  IE   1      2020-06-18 14:42:00
			2       1        /foo         0      Firefox 1  Linux   00112233445566778899aabbccddef02  0         NULL   NULL  US   1      2020-06-18 14:42:00
			3       1        /foo         0      Firefox 1  Linux   00112233445566778899aabbccddef01  0         NULL   NULL  IE   0      2020-06-18 14:42:00
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
			hit_id  site_id  path  title  event  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                       00112233445566778899aabbccddef01  0         NULL   NULL       1      2020-06-18 14:42:00
			2       1        /foo         0                       00112233445566778899aabbccddef02  0         NULL   NULL       1      2020-06-18 14:42:00
			3       1        /foo         0                       00112233445566778899aabbccddef01  0         NULL   NULL       0      2020-06-18 14:42:00
			`,
		},

		// Don't persist if session is blank.
		{
			APICountRequest{Hits: []APICountRequestHit{
				{Path: "/foo", Session: "a"},
				{Path: "/foo"},
			}},
			400, `{"errors":{"1":"session or browser/IP not set; use no_sessions if you don't want to track unique visits"}}`, `
			hit_id  site_id  path  title  event  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                       00112233445566778899aabbccddef01  0         NULL   NULL       1      2020-06-18 14:42:00
			`,
		},

		// Filter bots
		{
			APICountRequest{NoSessions: true, Hits: []APICountRequestHit{
				{Path: "/foo"},
				{Path: "/foo", UserAgent: "curl/7.8"},
			}},
			202, respOK, `
			hit_id  site_id  path  title  event  browser   system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                        00112233445566778899aabbccddef01  0         NULL   NULL       1      2020-06-18 14:42:00
			2       1        /foo         0      curl 7.8          00112233445566778899aabbccddef02  7         NULL   NULL       1      2020-06-18 14:42:00
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
			hit_id  site_id  path  title  event  browser  system  session                           bot  ref  ref_s  size  loc  first  created_at
			1       1        /foo         0                       00112233445566778899aabbccddef01  0         NULL   NULL  AU   1      2020-06-18 14:42:00
			`,
		},
	}

	ztime.SetNow(t, "2020-06-18 14:42:00")
	perm := goatcounter.APIPermCount

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := gctest.DB(t)
			site := Site(ctx)
			site.Settings.Collect.Set(goatcounter.CollectHits)
			site.Settings.IgnoreIPs = []string{"1.1.1.1"}
			err := site.Update(ctx)
			if err != nil {
				t.Fatal(err)
			}

			r, rr := newAPITest(ctx, t, "POST", "/api/v0/count",
				bytes.NewReader(zjson.MustMarshal(tt.body)), perm)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)
			if d := ztest.Diff(rr.Body.String(), tt.wantRet, ztest.DiffJSON); d != "" {
				t.Errorf("\nout:  %s\nwant: %s", rr.Body.String(), tt.wantRet)
			}

			gctest.StoreHits(ctx, t, false)

			tt.want = strings.TrimSpace(strings.ReplaceAll(tt.want, "\t", ""))
			have := strings.TrimSpace(zdb.DumpString(ctx, `
				select
					hits.hit_id,
					hits.site_id,

					paths.path,
					paths.title,
					paths.event,

					browsers.name || ' ' || browsers.version as browser,
					systems.name  || ' ' || systems.version  as system,

					hits.session,
					hits.bot,
					refs.ref,
					refs.ref_scheme as ref_s,
					sizes.size,
					hits.location as loc,
					hits.first_visit as first,
					hits.created_at
				from hits
				join paths          using (path_id)
				left join refs      using (ref_id)
				left join sizes     using (size_id)
				join browsers using (browser_id)
				join systems  using (system_id)
				order by hit_id asc`))
			if strings.Count(have, "\n") == 0 { // No data, only the header.
				have = ""
			}

			if d := ztest.Diff(have, tt.want); d != "" {
				t.Error(d)
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

func TestAPIPaths(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:13:14")

	many := func(ctx context.Context, t *testing.T) {
		p := make(goatcounter.Paths, 50)
		for i := range p {
			c := strconv.Itoa(i + 1)
			p[i].Site = 1
			p[i].Path = "/" + c
			p[i].Title = c + " - " + c
			err := p[i].GetOrInsert(ctx)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	tests := []struct {
		name     string
		setup    func(context.Context, *testing.T)
		query    string
		wantCode int
		want     string
	}{
		{"no paths",
			nil, "", 200, `{"more": false, "paths": []}`},

		{"works",
			func(ctx context.Context, t *testing.T) {
				for _, p := range []goatcounter.Path{
					{Site: 1, Path: "/a", Title: "Hello"},
				} {
					err := p.GetOrInsert(ctx)
					if err != nil {
						t.Fatal(err)
					}
				}
			}, "", 200, `{
            "more": false,
            "paths": [
                {"event": false, "id": 1, "path": "/a", "title": "Hello"}
            ]}`,
		},

		{"paginates",
			func(ctx context.Context, t *testing.T) {
				many(ctx, t)
			}, "", 200, `{
			"more": true,
			"paths": [
				{"event": false, "id": 1, "path": "/1", "title": "1 - 1"},
				{"event": false, "id": 2, "path": "/2", "title": "2 - 2"},
				{"event": false, "id": 3, "path": "/3", "title": "3 - 3"},
				{"event": false, "id": 4, "path": "/4", "title": "4 - 4"},
				{"event": false, "id": 5, "path": "/5", "title": "5 - 5"},
				{"event": false, "id": 6, "path": "/6", "title": "6 - 6"},
				{"event": false, "id": 7, "path": "/7", "title": "7 - 7"},
				{"event": false, "id": 8, "path": "/8", "title": "8 - 8"},
				{"event": false, "id": 9, "path": "/9", "title": "9 - 9"},
				{"event": false, "id": 10, "path": "/10", "title": "10 - 10"},
				{"event": false, "id": 11, "path": "/11", "title": "11 - 11"},
				{"event": false, "id": 12, "path": "/12", "title": "12 - 12"},
				{"event": false, "id": 13, "path": "/13", "title": "13 - 13"},
				{"event": false, "id": 14, "path": "/14", "title": "14 - 14"},
				{"event": false, "id": 15, "path": "/15", "title": "15 - 15"},
				{"event": false, "id": 16, "path": "/16", "title": "16 - 16"},
				{"event": false, "id": 17, "path": "/17", "title": "17 - 17"},
				{"event": false, "id": 18, "path": "/18", "title": "18 - 18"},
				{"event": false, "id": 19, "path": "/19", "title": "19 - 19"},
				{"event": false, "id": 20, "path": "/20", "title": "20 - 20"}
			]}`,
		},

		{"paginates",
			func(ctx context.Context, t *testing.T) {
				many(ctx, t)
			}, "after=19&limit=5", 200, `{
			"more": true,
			"paths": [
				{"event": false, "id": 20, "path": "/20", "title": "20 - 20"},
				{"event": false, "id": 21, "path": "/21", "title": "21 - 21"},
				{"event": false, "id": 22, "path": "/22", "title": "22 - 22"},
				{"event": false, "id": 23, "path": "/23", "title": "23 - 23"},
				{"event": false, "id": 24, "path": "/24", "title": "24 - 24"}
			]}`,
		},

		{"paginates at end",
			func(ctx context.Context, t *testing.T) {
				many(ctx, t)
			}, "after=45&limit=5", 200, `{
			"more": false,
			"paths": [
				{"event": false, "id": 46, "path": "/46", "title": "46 - 46"},
				{"event": false, "id": 47, "path": "/47", "title": "47 - 47"},
				{"event": false, "id": 48, "path": "/48", "title": "48 - 48"},
				{"event": false, "id": 49, "path": "/49", "title": "49 - 49"},
				{"event": false, "id": 50, "path": "/50", "title": "50 - 50"}
			]}`,
		},

		{"after higher than result",
			func(ctx context.Context, t *testing.T) {
				many(ctx, t)
			}, "after=50&limit=5", 200, `{
			"more": false,
			"paths": []}`,
		},
	}

	perm := goatcounter.APIPermStats
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := gctest.DB(t)
			if tt.setup != nil {
				tt.setup(ctx, t)
			}

			r, rr := newAPITest(ctx, t, "GET", "/api/v0/paths?"+tt.query, nil, perm)
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			if d := ztest.Diff(rr.Body.String(), tt.want, ztest.DiffJSON); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestAPIHits(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:13:14")

	many := func(ctx context.Context, t *testing.T) {
		h := make(goatcounter.Hits, 50)
		for i := range h {
			c := strconv.Itoa(i + 1)
			h[i].Path = "/" + c
			h[i].Site = 1
			h[i].Title = "title - " + c
			h[i].FirstVisit = true
		}

		gctest.StoreHits(ctx, t, false, h...)
	}

	tests := []struct {
		name     string
		query    string
		wantCode int
		setup    func(context.Context, *testing.T)
		want     string
	}{
		{"no hits", "", 200, nil, `{"more": false, "total": 0, "hits": []}`},

		{"works", "limit=3", 200,
			func(ctx context.Context, t *testing.T) { many(ctx, t) }, `{
			"more": true,
			"total": 3,
			"hits": [{
				"count":  1,
				"event":         false,
				"max":           1,
				"path":          "/50",
				"path_id":       50,
				"title":         "title - 50",
				"stats": [{
					"daily":   0,
					"day":            "2020-06-11",
					"hourly":  [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-12",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-13",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-14",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-15",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-16",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-17",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 1,
					"day": "2020-06-18",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}]
			}, {
				"count": 1,
				"event": false,
				"max": 1,
				"path": "/49",
				"path_id": 49,
				"title": "title - 49",
				"stats": [{
					"daily": 0,
					"day": "2020-06-11",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-12",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-13",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-14",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-15",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-16",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-17",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 1,
					"day": "2020-06-18",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}]
			}, {
				"count": 1,
				"event": false,
				"max": 1,
				"path": "/48",
				"path_id": 48,
				"title": "title - 48",
				"stats": [{
					"daily": 0,
					"day": "2020-06-11",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-12",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-13",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-14",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-15",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-16",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-17",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 1,
					"day": "2020-06-18",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}]
			}]
		}`},

		{"exclude", "limit=1&exclude_paths=50,49&daily=true&start=2020-06-17&end=2020-06-19", 200,
			func(ctx context.Context, t *testing.T) { many(ctx, t) }, `{
			"more": true,
			"total": 1,
			"hits": [{
				"count": 1,
				"event": false,
				"max": 1,
				"path": "/48",
				"path_id": 48,
				"title": "title - 48",
				"stats": [{
					"daily": 0,
					"day": "2020-06-17",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 1,
					"day": "2020-06-18",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-19",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}]
			}]
		}`},

		{"include", "limit=1&exclude_paths=&include_paths=10&daily=true&start=2020-06-17&end=2020-06-19", 200,
			func(ctx context.Context, t *testing.T) { many(ctx, t) }, `{
			"more": false,
			"total": 1,
			"hits": [{
				"count": 1,
				"event": false,
				"max": 1,
				"path": "/10",
				"path_id": 10,
				"title": "title - 10",
				"stats": [{
					"daily": 0,
					"day": "2020-06-17",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 1,
					"day": "2020-06-18",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}, {
					"daily": 0,
					"day": "2020-06-19",
					"hourly": [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
				}]
			}]
		}`},
	}

	perm := goatcounter.APIPermStats
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := gctest.DB(t)
			if tt.setup != nil {
				tt.setup(ctx, t)
			}

			r, rr := newAPITest(ctx, t, "GET", "/api/v0/stats/hits?"+tt.query, nil, perm)
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			if d := ztest.Diff(rr.Body.String(), tt.want, ztest.DiffJSON); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestAPIStats(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:13:14")

	many := func(ctx context.Context, t *testing.T) {
		h := make(goatcounter.Hits, 50)
		for i := range h {
			c := strconv.Itoa(i + 1)
			h[i].Path = "/" + c
			h[i].Site = 1
			h[i].Title = "title - " + c
			h[i].FirstVisit = true
			if i < 20 || i%2 == 0 {
				h[i].UserAgentHeader = fmt.Sprintf(
					"Mozilla/5.0 (X11; Linux x86_64; rv:%[1]d.0) Gecko/20100101 Firefox/%[1]d.0", i)
			} else {
				h[i].UserAgentHeader = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
					"(KHTML, like Gecko) Chrome/106.0.0.0 Safari/537.36"
			}
		}

		gctest.StoreHits(ctx, t, false, h...)
	}

	tests := []struct {
		name     string
		page     string
		query    string
		wantCode int
		setup    func(context.Context, *testing.T)
		want     string
	}{
		{"no hits", "browsers", "", 200, nil, `{"more": false, "stats": []}`},

		{"works", "browsers", "", 200,
			func(ctx context.Context, t *testing.T) { many(ctx, t) },
			`{
				"more": false,
				"stats": [
					{"count": 35, "id": "Firefox", "name": "Firefox"},
					{"count": 15, "id": "Chrome", "name": "Chrome"}
				]
			}`},
	}

	perm := goatcounter.APIPermStats
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := gctest.DB(t)
			if tt.setup != nil {
				tt.setup(ctx, t)
			}

			r, rr := newAPITest(ctx, t, "GET", "/api/v0/stats/"+tt.page+"?"+tt.query, nil, perm)
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			if d := ztest.Diff(rr.Body.String(), tt.want, ztest.DiffJSON); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestAPIStatsDetail(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:13:14")

	many := func(ctx context.Context, t *testing.T) {
		h := make(goatcounter.Hits, 50)
		for i := range h {
			c := strconv.Itoa(i + 1)
			h[i].Path = "/" + c
			h[i].Site = 1
			h[i].Title = "title - " + c
			h[i].FirstVisit = true
			if i < 20 || i%2 == 0 {
				h[i].UserAgentHeader = fmt.Sprintf(
					"Mozilla/5.0 (X11; Linux x86_64; rv:%[1]d.0) Gecko/20100101 Firefox/%[1]d.0", i)
			} else {
				h[i].UserAgentHeader = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
					"(KHTML, like Gecko) Chrome/106.0.0.0 Safari/537.36"
			}
		}

		gctest.StoreHits(ctx, t, false, h...)
	}

	tests := []struct {
		name     string
		page     string
		query    string
		wantCode int
		setup    func(context.Context, *testing.T)
		want     string
	}{
		{"no hits", "browsers/Firefox", "", 200, nil, `{"more": false, "stats": []}`},

		{"works", "browsers/Firefox", "limit=3", 200,
			func(ctx context.Context, t *testing.T) { many(ctx, t) },
			`{
				"more": true,
				"stats": [
					{"count": 1, "name": "Firefox 0"},
					{"count": 1, "name": "Firefox 1"},
					{"count": 1, "name": "Firefox 10"}
				]
			}`},
	}

	perm := goatcounter.APIPermStats
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := gctest.DB(t)
			if tt.setup != nil {
				tt.setup(ctx, t)
			}

			r, rr := newAPITest(ctx, t, "GET", "/api/v0/stats/"+tt.page+"?"+tt.query, nil, perm)
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, tt.wantCode)

			if d := ztest.Diff(rr.Body.String(), tt.want, ztest.DiffJSON); d != "" {
				t.Error(d)
			}
		})
	}
}

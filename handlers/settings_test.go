package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/goatcounter/v2/pkg/bgrun"
	"zgo.at/zdb"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
)

func TestSettingsTpl(t *testing.T) {
	tests := []handlerTest{
		{
			setup: func(ctx context.Context, t *testing.T) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
					{FirstVisit: true, Site: 1, Path: "/asd", Title: "AAA", CreatedAt: now},
					{FirstVisit: true, Site: 1, Path: "/asd", Title: "AAA", CreatedAt: now},
					{FirstVisit: true, Site: 1, Path: "/zxc", Title: "BBB", CreatedAt: now},
				}...)
			},
			router:   newBackend,
			path:     "/settings/purge?path=/asd",
			auth:     true,
			wantCode: 200,
			wantBody: "<tr><td>2</td><td>/asd</td><td>AAA</td></tr>",
		},

		{
			setup: func(ctx context.Context, t *testing.T) {
				one := int64(1)
				ss := goatcounter.Site{
					Code:   "subsite",
					Parent: &one,
				}
				err := ss.Insert(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:   newBackend,
			path:     "/settings/sites/remove/2",
			auth:     true,
			wantCode: 200,
			wantBody: "Are you sure you want to remove the site",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestSettingsPurge(t *testing.T) {
	t.Skip() // Fails after we stopped storing hits.

	tests := []handlerTest{
		{
			setup: func(ctx context.Context, t *testing.T) {
				now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)
				gctest.StoreHits(ctx, t, false, []goatcounter.Hit{
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/asd", CreatedAt: now},
					{Site: 1, Path: "/zxc", CreatedAt: now},
				}...)
			},
			router:       newBackend,
			path:         "/settings/purge",
			body:         map[string]string{"path": "/asd", "paths": "1,"},
			method:       "POST",
			auth:         true,
			wantFormCode: 303,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			bgrun.Wait("")

			var hits goatcounter.Hits
			err := hits.TestList(r.Context(), false)
			if err != nil {
				t.Fatal(err)
			}
			if len(hits) != 1 {
				t.Errorf("%d hits in DB; expected 1:\n%v", len(hits), zdb.DumpString(r.Context(), `select * from hits`))
			}
		})
	}
}

func TestSettingsSitesAdd(t *testing.T) {
	t.Skip()

	tests := []handlerTest{
		{
			name:         "new site",
			setup:        func(ctx context.Context, t *testing.T) {},
			router:       newBackend,
			path:         "/settings/sites/add",
			body:         map[string]string{"cname": "add.example.com", "code": "add"},
			method:       "POST",
			auth:         true,
			wantFormCode: 303,
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   1       a`,
		},
		{
			name: "already exists for this account",
			setup: func(ctx context.Context, t *testing.T) {
				s := goatcounter.Site{
					Parent: ztype.Ptr(int64(1)),
					Cname:  ztype.Ptr("add.example.com"),
					Code:   "add",
				}
				err := s.Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
			},
			router:       newBackend,
			path:         "/settings/sites/add",
			body:         map[string]string{"cname": "add.example.com", "code": "add"},
			method:       "POST",
			auth:         true,
			wantFormCode: 400,
			wantFormBody: "already exists",
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   1       a`,
		},
		{
			name: "already exists on other account",
			setup: func(ctx context.Context, t *testing.T) {
				s := goatcounter.Site{
					Cname: ztype.Ptr("add.example.com"),
					Code:  "add",
				}
				err := s.Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
			},
			router:       newBackend,
			path:         "/settings/sites/add",
			body:         map[string]string{"cname": "add.example.com", "code": "add"},
			method:       "POST",
			auth:         true,
			wantFormCode: 400,
			wantFormBody: "already exists",
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   NULL    a`,
		},
		{
			name: "undelete",
			setup: func(ctx context.Context, t *testing.T) {
				s := goatcounter.Site{
					Parent: ztype.Ptr(int64(1)),
					Cname:  ztype.Ptr("add.example.com"),
					Code:   "add",
				}
				err := s.Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
				err = s.Delete(ctx, false)
				if err != nil {
					t.Fatal(err)
				}
			},
			router:       newBackend,
			path:         "/settings/sites/add",
			body:         map[string]string{"cname": "add.example.com", "code": "add"},
			method:       "POST",
			auth:         true,
			wantFormCode: 303,
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   1       a`,
		},
		{
			name: "undelete other account",
			setup: func(ctx context.Context, t *testing.T) {
				s := goatcounter.Site{
					Cname: ztype.Ptr("add.example.com"),
					Code:  "add",
				}
				err := s.Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
				err = s.Delete(ctx, false)
				if err != nil {
					t.Fatal(err)
				}
			},
			router:       newBackend,
			path:         "/settings/sites/add",
			body:         map[string]string{"cname": "add.example.com", "code": "add"},
			method:       "POST",
			auth:         true,
			wantFormCode: 400,
			wantFormBody: "already exists",
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   NULL    d`,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			have := zdb.DumpString(r.Context(), `select site_id, substr(code, 0, 6) as code, cname, parent, state from sites`)
			if d := zdb.Diff(have, tt.want); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestSettingsSitesRemove(t *testing.T) {
	t.Skip()

	tests := []handlerTest{
		{
			name: "remove",
			setup: func(ctx context.Context, t *testing.T) {
				err := (&goatcounter.Site{
					Parent: ztype.Ptr(int64(1)),
					Cname:  ztype.Ptr("add.example.com"),
					Code:   "add",
				}).Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
			},
			router:       newBackend,
			path:         "/settings/sites/remove/2",
			body:         map[string]string{"cname": "add.example.com"},
			method:       "POST",
			auth:         true,
			wantFormCode: 303,
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   1       d`,
		},
		{
			name:         "remove self",
			setup:        func(ctx context.Context, t *testing.T) {},
			router:       newBackend,
			path:         "/settings/sites/remove/1",
			body:         map[string]string{"cname": "add.example.com"},
			method:       "POST",
			auth:         true,
			wantFormCode: 303,
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    d`,
		},
		{
			name: "remove other account",
			setup: func(ctx context.Context, t *testing.T) {
				s := goatcounter.Site{
					Cname: ztype.Ptr("add.example.com"),
					Code:  "add",
				}
				err := s.Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
			},
			router:       newBackend,
			path:         "/settings/sites/remove/2",
			body:         map[string]string{"cname": "add.example.com"},
			method:       "POST",
			auth:         true,
			wantFormCode: 404,
			want: `
				site_id  code   cname             parent  state
				1        gctes  gctest.localhost  NULL    a
				2        serve  add.example.com   NULL    a`,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			have := zdb.DumpString(r.Context(), `select site_id, substr(code, 0, 6) as code, cname, parent, state from sites`)
			if d := zdb.Diff(have, tt.want); d != "" {
				t.Error(d)
			}
		})
	}
}

func TestSettingsMerge(t *testing.T) {
	do := func(ctx context.Context, t *testing.T, params map[string]string) {
		form := formBody(params)
		r, rr := newTest(ctx, "POST", "/settings/merge", strings.NewReader(form))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		login(t, r)
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 303)
		bgrun.Wait("")
	}

	check := func(ctx context.Context, t *testing.T, want string) {
		t.Helper()
		have := new(strings.Builder)
		for _, q := range []string{
			`select * from paths         order by path_id`,
			`select * from hit_counts    order by path_id`,
			`select * from hit_stats     order by path_id`,
			`select * from browser_stats order by path_id`,
			`select * from system_stats  join systems using(system_id) order by path_id`,
		} {
			zdb.Dump(ctx, have, q)
		}

		if d := ztest.Diff(have.String(), want, ztest.DiffNormalizeWhitespace); d != "" {
			t.Error(d)
		}
	}

	var (
		uaLinux = `Mozilla/5.0 (X11; Linux x86_64; rv:139.0) Gecko/20100101 Firefox/139.0`
		uaMac   = `Mozilla/5.0 (Macintosh; Intel Mac OS X 10.14; rv:139.0) Gecko/20100101 Firefox/139.0`
		uaWin   = `Mozilla/5.0 (Windows NT 10.0; WOW64; rv:139.0) Gecko/20100101 Firefox/139.0`
		now     = ztime.FromString("2025-06-13 12:13:40")
	)

	t.Run("merge one path", func(t *testing.T) {
		ctx := gctest.DB(t)
		gctest.StoreHits(ctx, t, false,
			goatcounter.Hit{FirstVisit: true, CreatedAt: now, Path: "/one", UserAgentHeader: uaLinux},
			goatcounter.Hit{FirstVisit: true, CreatedAt: now, Path: "/two", UserAgentHeader: uaMac},
			goatcounter.Hit{FirstVisit: true, CreatedAt: now, Path: "/three", UserAgentHeader: uaWin})

		do(ctx, t, map[string]string{
			"merge_with": "1",
			"paths":      "2",
		})
		check(ctx, t, `
			path_id  site_id  path    title  event
			1        1        /one           0
			3        1        /three         0

			site_id  path_id  hour                 total
			1        1        2025-06-13 12:00:00  2
			1        3        2025-06-13 12:00:00  1

			site_id  path_id  day                  stats
			1        1        2025-06-13 00:00:00  [0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0,0,0,0,0,0,0,0]
			1        3        2025-06-13 00:00:00  [0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0]

			site_id  path_id  browser_id  day                  count
			1        1        1           2025-06-13 00:00:00  2
			1        3        1           2025-06-13 00:00:00  1

			site_id  path_id  system_id  day                  count  name     version
            1        1        1          2025-06-13 00:00:00  1      Linux
            1        1        2          2025-06-13 00:00:00  1      macOS    10.14
            1        3        3          2025-06-13 00:00:00  1      Windows  10
		`)
	})

	t.Run("merge two paths", func(t *testing.T) {
		ctx := gctest.DB(t)
		gctest.StoreHits(ctx, t, false,
			goatcounter.Hit{FirstVisit: true, CreatedAt: now, Path: "/one", UserAgentHeader: uaLinux},
			goatcounter.Hit{FirstVisit: true, CreatedAt: now, Path: "/two", UserAgentHeader: uaMac},
			goatcounter.Hit{FirstVisit: true, CreatedAt: now, Path: "/three", UserAgentHeader: uaWin})

		do(ctx, t, map[string]string{
			"merge_with": "1",
			"paths":      "2,3",
		})
		check(ctx, t, `
			path_id  site_id  path  title  event
			1        1        /one         0

			site_id  path_id  hour                 total
			1        1        2025-06-13 12:00:00  3

			site_id  path_id  day                  stats
			1        1        2025-06-13 00:00:00  [0,0,0,0,0,0,0,0,0,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0]

			site_id  path_id  browser_id  day                  count
			1        1        1           2025-06-13 00:00:00  3

			site_id  path_id  system_id  day                  count  name     version
            1        1        1          2025-06-13 00:00:00  1      Linux
            1        1        2          2025-06-13 00:00:00  1      macOS    10.14
            1        1        3          2025-06-13 00:00:00  1      Windows  10
		`)
	})
}

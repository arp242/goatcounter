// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestBackendTpl(t *testing.T) {
	tests := []struct {
		page, want string
	}{
		{"/help/start", "Getting started"},

		// rdr
		// {"/api", "Backend integration"},

		// User settings
		{"/user/dashboard", "Paths overview"},
		{"/user/pref", "Your email"},
		{"/user/auth", "Change password"},
		{"/user/api", "API documentation"},

		// Settings
		{"/settings/main", "Data retention in days"},
		{"/settings/sites", "Copy all settings from the current site except the domain name"},
		{"/settings/users", "Access"},
		{"/settings/users/add", "Password"},
		{"/settings/users/1", "Password"},
		{"/settings/purge", "Remove or merge pageviews"},
		{"/settings/export", "format of the CSV file"},
		{"/settings/delete-account", "The site and all associated data will be permanently removed"},
		{"/settings/change-code", "Change your site code and login domain"},

		// Shared
		//{"/help", "I don’t see my pageviews?"},
		//{"/gdpr", "consult a lawyer"},
		{"/contact", "Send message"},
		{"/contribute", "Contribute"},
		{"/api.html", "Endpoints"},
		{"/api2.html", "<rapi-doc"},
		{"/api.json", `"consumes"`},
	}

	for _, tt := range tests {
		t.Run(tt.page, func(t *testing.T) {
			ctx := gctest.DB(t)
			site := Site(ctx)

			r, rr := newTest(ctx, "GET", tt.page, nil)
			r.Host = site.Code + "." + goatcounter.Config(ctx).Domain
			login(t, r)

			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 200)

			if !strings.Contains(rr.Body.String(), tt.want) {
				t.Errorf("doesn't contain %q in: %s", tt.want, rr.Body.String())
			}
		})
	}
}

func TestBackendPagesMore(t *testing.T) {
	ctx := gctest.DB(t)
	site := Site(ctx)
	now := ztime.Now()

	gctest.StoreHits(ctx, t, false,
		goatcounter.Hit{FirstVisit: true, Path: "/1"},
		goatcounter.Hit{FirstVisit: true, Path: "/2"},
		goatcounter.Hit{FirstVisit: true, Path: "/3"},
		goatcounter.Hit{FirstVisit: true, Path: "/4"},
		goatcounter.Hit{FirstVisit: true, Path: "/5"},
		goatcounter.Hit{FirstVisit: true, Path: "/6"},
		goatcounter.Hit{FirstVisit: true, Path: "/7"},
		goatcounter.Hit{FirstVisit: true, Path: "/8"},
		goatcounter.Hit{FirstVisit: true, Path: "/9"},
		goatcounter.Hit{FirstVisit: true, Path: "/10"},
	)
	url := fmt.Sprintf(
		"/load-widget?widget=0&exclude=1,2,3,4,5&max=10&period-start=%s&period-end=%s",
		now.Format("2006-01-02"), now.Format("2006-01-02"))

	r, rr := newTest(ctx, "GET", url, nil)
	r.Host = site.Code + "." + goatcounter.Config(ctx).Domain
	login(t, r)
	newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
	ztest.Code(t, rr, 200)

	var body map[string]any
	zjson.MustUnmarshal(rr.Body.Bytes(), &body)

	haveHTML := grep("tr id=", string(body["html"].(string)))
	wantHTML := `
        <tr id="/10" data-id="10" data-count="1"
        <tr id="/9" data-id="9" data-count="1"
        <tr id="/8" data-id="8" data-count="1"
        <tr id="/7" data-id="7" data-count="1"
        <tr id="/6" data-id="6" data-count="1"`

	delete(body, "html")
	haveJSON := string(zjson.MustMarshalIndent(body, "", "\t"))
	wantJSON := `{
		"max": 10,
		"more": false,
		"total_display": 5
	}`

	if d := ztest.Diff(haveHTML, wantHTML, ztest.DiffNormalizeWhitespace); d != "" {
		t.Error(d)
	}
	if d := ztest.Diff(haveJSON, wantJSON, ztest.DiffNormalizeWhitespace); d != "" {
		t.Error(d)
	}
}

func TestServeNewSite(t *testing.T) {
	emptySite := func(t *testing.T) context.Context {
		ctx := gctest.DB(t)
		if err := zdb.Exec(ctx, `delete from sites`); err != nil {
			t.Fatal(err)
		}
		if err := zdb.Exec(ctx, `delete from users`); err != nil {
			t.Fatal(err)
		}
		return ctx
	}

	t.Run("form serve", func(t *testing.T) {
		ctx := emptySite(t)
		goatcounter.Config(ctx).GoatcounterCom = false

		r, rr := newTest(ctx, "GET", "/", nil)
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)

		if !strings.Contains(rr.Body.String(), `Create your first site and user`) {
			t.Errorf("wrong body:\n\n%s", rr.Body)
		}
	})
	t.Run("form saas", func(t *testing.T) {
		ctx := emptySite(t)

		r, rr := newTest(ctx, "GET", "/", nil)
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 400)

		if !strings.Contains(rr.Body.String(), `no site at this domain`) {
			t.Errorf("wrong body:\n\n%s", rr.Body)
		}
	})

	t.Run("submit serve", func(t *testing.T) {
		ctx := emptySite(t)
		goatcounter.Config(ctx).GoatcounterCom = false

		body, contentType, err := ztest.MultipartForm(map[string]string{
			"email":     "new@example.com",
			"password":  "secretsecret",
			"password2": "secretsecret",
		})
		if err != nil {
			t.Fatal(err)
		}

		r, rr := newTest(ctx, "POST", "/", body)
		r.Header.Set("Content-Type", contentType)
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 303)

		have := zdb.DumpString(ctx, `select site_id, cname from sites`)
		want := "site_id  cname\n2        goatcounter.localhost\n"
		if have != want {
			t.Errorf("\nhave:\n%s\nwant:\n%s", have, want)
		}
	})

	t.Run("submit saas", func(t *testing.T) {
		ctx := emptySite(t)

		body, contentType, err := ztest.MultipartForm(map[string]string{
			"email":     "new@example.com",
			"password":  "secretsecret",
			"password2": "secretsecret",
		})
		if err != nil {
			t.Fatal(err)
		}

		r, rr := newTest(ctx, "POST", "/", body)
		r.Header.Set("Content-Type", contentType)
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 303)

		have := zdb.DumpString(ctx, `select site_id, cname from sites`)
		want := "site_id  cname\n"
		if have != want {
			t.Errorf("\nhave:\n%s\nwant:\n%s", have, want)
		}
	})
}

func grep(pat, lines string) string {
	s := strings.Split(lines, "\n")
	r := make([]string, 0, len(s)/2)
	re := regexp.MustCompile(pat)
	for _, l := range s {
		if re.MatchString(l) {
			r = append(r, l)
		}
	}
	return strings.Join(r, "\n")
}

func BenchmarkCount(b *testing.B) {
	ctx := gctest.DB(b)

	r, rr := newTest(ctx, "GET", "/count", nil)
	r.URL.RawQuery = url.Values{
		"p": {"/test.html"},
		"t": {"Benchmark test for /count"},
		"r": {"https://example.com/foo"},
	}.Encode()
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:72.0) Gecko/20100101 Firefox/72.0")
	r.Header.Set("Referer", "https://example.com/foo")

	handler := newBackend(zdb.MustGetDB(ctx)).ServeHTTP

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(rr, r)
	}
}

func newBackend(db zdb.DB) chi.Router {
	return NewBackend(db, nil, true, true, false, "example.com", "", 10, 0)
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/ztest"
	"zgo.at/zvalidate"
)

func newAPITest(
	t *testing.T, method, path string, body io.Reader, perm goatcounter.APITokenPermissions,
) (
	context.Context, func(), *http.Request, *httptest.ResponseRecorder,
) {
	ctx, clean := gctest.DB(t)

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

	return ctx, clean, r, rr
}

func TestAPIBasics(t *testing.T) {
	t.Run("error", func(t *testing.T) {
		t.Run("no-auth", func(t *testing.T) {
			ctx, clean, r, rr := newAPITest(t, "GET", "/api/v0/test", nil, goatcounter.APITokenPermissions{})
			defer clean()

			delete(r.Header, "Authorization")
			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"no Authorization header"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("wrong-auth", func(t *testing.T) {
			ctx, clean, r, rr := newAPITest(t, "GET", "/api/v0/test", nil, goatcounter.APITokenPermissions{})
			defer clean()

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
			ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test", body, goatcounter.APITokenPermissions{})
			defer clean()

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 403)

			want := `{"error":"requires [count export] permissions"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("404", func(t *testing.T) {
			ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/doesnt-exist", nil, goatcounter.APITokenPermissions{})
			defer clean()

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 404)

			want := `{"error":"Not Found"}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("500", func(t *testing.T) {
			ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test",
				strings.NewReader(`{"status":500}`),
				goatcounter.APITokenPermissions{})
			defer clean()

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 500)

			want := `{"error":"unexpected error code ‘`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("panic", func(t *testing.T) {
			ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test",
				strings.NewReader(`{"panic":true}`),
				goatcounter.APITokenPermissions{})
			defer clean()

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 500)

			want := `{"error":"unexpected error code ‘`
			if !strings.HasPrefix(rr.Body.String(), want) {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})

		t.Run("ct", func(t *testing.T) {
			ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test", nil, goatcounter.APITokenPermissions{})
			defer clean()

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

			ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test",
				bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
					"validate": v,
				})),
				goatcounter.APITokenPermissions{})
			defer clean()

			newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 400)

			want := `{"errors":{"e":["must be a valid email address"],"r":["must be set"]}}`
			if rr.Body.String() != want {
				t.Errorf("\nwant: %s\ngot:  %s\n", want, rr.Body.String())
			}
		})
	})

	t.Run("no-perm", func(t *testing.T) {
		ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test", nil, goatcounter.APITokenPermissions{})
		defer clean()

		newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
	})

	t.Run("check-perm", func(t *testing.T) {
		body := bytes.NewReader(zjson.MustMarshal(map[string]interface{}{
			"perm": goatcounter.APITokenPermissions{Export: true, Count: true},
		}))
		ctx, clean, r, rr := newAPITest(t, "POST", "/api/v0/test", body, goatcounter.APITokenPermissions{
			Export: true, Count: true,
		})
		defer clean()

		newBackend(zdb.MustGet(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
	})
}

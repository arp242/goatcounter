// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

func TestUserNew(t *testing.T) {
	tests := []handlerTest{
		{
			name:         "basic",
			router:       newBackend,
			path:         "/user/new",
			wantCode:     200,
			wantFormCode: 200,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestUserRequestLogin(t *testing.T) {
	tests := []handlerTest{
		{
			name: "basic",
			setup: func(ctx context.Context) {
				user := goatcounter.User{Site: 1, Name: "new site", Email: "new@example.com"}
				err := user.Insert(ctx)
				if err != nil {
					panic(err)
				}
			},
			router:       newBackend,
			method:       "POST",
			path:         "/user/requestlogin",
			body:         map[string]string{"email": "new@example.com"},
			wantCode:     303,
			wantFormCode: 303,
		},
		{
			name:         "nonexistent",
			router:       newBackend,
			method:       "POST",
			path:         "/user/requestlogin",
			body:         map[string]string{"email": "nonexistent@example.com"},
			wantCode:     303,
			wantFormCode: 303,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			msg := fmt.Sprintf("%v", zhttp.ReadFlash(rr, r))
			var want string
			if tt.name == "basic" {
				want = `&{i All good. Login URL emailed to "new@example.com"; please click it in the next hour to continue.`
			} else {
				want = `&{e Not an account on this site: "nonexistent@example.com"}`
			}

			if !strings.HasPrefix(msg, want) {
				t.Errorf("wrong flash\nwant: %q\nout:  %q", want, msg)
			}
		})
	}
}

func TestUserLogin(t *testing.T) {
	tests := []handlerTest{
		{
			name: "basic",
			setup: func(ctx context.Context) {
				user := goatcounter.User{Site: 1, Name: "new site", Email: "new@example.com"}
				err := user.Insert(ctx)
				if err != nil {
					panic(err)
				}

				_, err = zdb.MustGet(ctx).ExecContext(ctx, `update users set
					login_request='asdf', login_at=current_timestamp
					where id=$1 and site=1`, user.ID)
				if err != nil {
					panic(err)
				}
			},
			router:       newBackend,
			path:         "/user/login/asdf",
			wantCode:     303,
			wantFormCode: 303,
		},

		{
			name:         "nonexistent",
			router:       newBackend,
			path:         "/user/login/nonexistent",
			wantCode:     403,
			wantFormCode: 403,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			// TODO: ensure we're actually logged in.
		})
	}
}

func TestUserLogout(t *testing.T) {
	t.Skip() // TODO
	tests := []handlerTest{
		{
			name:         "basic",
			method:       "POST",
			router:       newBackend,
			path:         "/user/logout",
			auth:         true,
			wantCode:     303,
			wantFormCode: 303,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			// TODO: ensure we're actually logged in.
		})
	}
}

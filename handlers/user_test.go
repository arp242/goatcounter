// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
)

func TestUserNew(t *testing.T) {
	tests := []handlerTest{
		{
			name:   "basic",
			router: newBackend,
			setup: func(ctx context.Context, t *testing.T) {
				u := goatcounter.User{Site: 1, Name: "Example", Email: "test@example.com", Password: []byte("coconuts")}
				err := u.Insert(ctx)
				if err != nil {
					t.Fatal(err)
				}
			},
			path:         "/user/new",
			wantCode:     200,
			wantFormCode: 200,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestUserLogin(t *testing.T) {
	tests := []handlerTest{
		{
			name: "basic",
			setup: func(ctx context.Context, t *testing.T) {
				user := goatcounter.User{Site: 1, Name: "new site", Email: "new@example.com", Password: []byte("coconuts")}
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
			// TODO: ensure we're actually logged out.
		})
	}
}

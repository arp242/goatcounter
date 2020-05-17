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
)

func TestUserNew(t *testing.T) {
	tests := []handlerTest{
		{
			name:   "basic",
			router: newBackend,
			setup: func(ctx context.Context, t *testing.T) {
				u := goatcounter.User{Site: 1, Email: "test@example.com", Password: []byte("coconuts")}
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
	tests := []handlerTest{}

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

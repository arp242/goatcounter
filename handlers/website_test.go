// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebsiteTpl(t *testing.T) {
	tests := []handlerTest{
		{
			name:     "index",
			router:   newWebsite,
			path:     "/",
			wantCode: 200,
			wantBody: "doesn’t track users with",
		},
		{
			name:     "help",
			router:   newWebsite,
			path:     "/help",
			wantCode: 200,
			wantBody: "I don’t see my pageviews?",
		},
		{
			name:     "privacy",
			router:   newWebsite,
			path:     "/privacy",
			wantCode: 200,
			wantBody: "Screen size",
		},
		{
			name:     "terms",
			router:   newWebsite,
			path:     "/terms",
			wantCode: 200,
			wantBody: "The “services” are any software, application, product, or service",
		},

		{
			name:     "status",
			router:   newWebsite,
			path:     "/status",
			wantCode: 200,
			wantBody: "uptime",
		},

		{
			name:     "signup",
			router:   newWebsite,
			path:     "/signup",
			wantCode: 200,
			wantBody: `<label for="email">Email address</label>`,
		},
	}

	for _, tt := range tests {
		runTest(t, tt, nil)
	}
}

func TestWebsiteSignup(t *testing.T) {
	tests := []handlerTest{
		{
			name:         "basic",
			method:       "POST",
			router:       newWebsite,
			path:         "/signup",
			body:         signupArgs{Code: "xxx", Email: "m@example.com", TuringTest: "9", Password: "coconuts"},
			wantCode:     303,
			wantFormCode: 303,
		},

		{
			name:         "no-code",
			method:       "POST",
			router:       newWebsite,
			path:         "/signup",
			body:         signupArgs{Email: "m@example.com", TuringTest: "9", Password: "coconuts"},
			wantCode:     200,
			wantBody:     "", // TODO: should return JSON
			wantFormCode: 200,
			wantFormBody: "Error: must be set, must be longer than 2 characters",
		},
	}

	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			// TODO: test state
		})
	}
}

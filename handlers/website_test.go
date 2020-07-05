// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
)

func TestWebsiteTpl(t *testing.T) {
	tests := []handlerTest{
		{
			name:     "index",
			router:   NewWebsite,
			path:     "/",
			wantCode: 200,
			wantBody: "doesn’t track users with",
		},
		{
			name:     "help",
			router:   NewWebsite,
			path:     "/help",
			wantCode: 200,
			wantBody: "I don’t see my pageviews?",
		},
		{
			name:     "privacy",
			router:   NewWebsite,
			path:     "/privacy",
			wantCode: 200,
			wantBody: "Screen size",
		},
		{
			name:     "terms",
			router:   NewWebsite,
			path:     "/terms",
			wantCode: 200,
			wantBody: "The “services” are any software, application, product, or service",
		},

		{
			name:     "status",
			router:   NewWebsite,
			path:     "/status",
			wantCode: 200,
			wantBody: "uptime",
		},

		{
			name:     "signup",
			router:   NewWebsite,
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
			router:       NewWebsite,
			path:         "/signup",
			body:         signupArgs{Code: "xxx", Email: "m@example.com", TuringTest: "9", Password: "coconuts"},
			wantCode:     303,
			wantFormCode: 303,
		},

		{
			name:         "no-code",
			method:       "POST",
			router:       NewWebsite,
			path:         "/signup",
			body:         signupArgs{Email: "m@example.com", TuringTest: "9", Password: "coconuts"},
			wantCode:     200,
			wantBody:     "", // TODO: should return JSON
			wantFormCode: 200,
			wantFormBody: "Error: must be set, must be longer than 2 characters",
		},
	}

	cfg.Plan = goatcounter.PlanPersonal
	for _, tt := range tests {
		runTest(t, tt, func(t *testing.T, rr *httptest.ResponseRecorder, r *http.Request) {
			// TODO: test state
		})
	}
}

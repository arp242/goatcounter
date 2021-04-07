// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"zgo.at/zdb"
)

func newWebsite(db zdb.DB) chi.Router { return NewWebsite(db, true) }

func TestWebsiteTpl(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"/", "doesn’t track users with"},
		{"/privacy", "Screen size"},
		{"/terms", "The “services” are any software, application, product, or service"},
		{"/why", "Footnotes"},
		{"/data", "CSV format with a header"},
		{"/design", "Firefox on iOS is just displayed as Safari"},
		{"/status", "uptime"},
		{"/signup", `<label for="email">Email address</label>`},
		{"/user/forgot", "Forgot domain"},

		{"/code/start", "Getting started"},

		// Shared

		// rdr
		// {"/api", "Backend integration"},

		{"/help", "I don’t see my pageviews?"},
		{"/gdpr", "consult a lawyer"},
		{"/contact", "Public Telegram Group"},
		{"/contribute", "One-time donation"},
		{"/api.html", "GoatCounter API documentation"},
		{"/api2.html", "<rapi-doc"},
		{"/api.json", `"description": "API for GoatCounter"`},
	}

	for _, tt := range tests {
		runTest(t, handlerTest{
			name:     tt.path,
			path:     tt.path,
			router:   newWebsite,
			wantCode: 200,
			wantBody: tt.want,
		}, nil)
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

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"zgo.at/zdb"
)

func newWebsite(ctx context.Context) chi.Router { return NewWebsite(zdb.MustGetDB(ctx), true) }

func TestWebsiteTpl(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"/", "doesn’t track users with"},
		{"/help/privacy", "for browsers it looks like"},
		{"/help/terms", "The “services” are any software, application, product, or service"},
		{"/why", "Footnotes"},
		{"/design", "Firefox on iOS is just displayed as Safari"},
		{"/help/translating", "translate GoatCounter"},
		{"/status", "uptime"},
		{"/signup", `<label for="email">Email address</label>`},
		{"/user/forgot", "Forgot domain"},

		{"/help/start", "Getting started"},

		// Shared

		// rdr
		// {"/api", "Backend integration"},

		//{"/help", "I don’t see my pageviews?"},
		{"/help/gdpr", "consult a lawyer"},
		{"/contact", "Send message"},
		{"/contribute", "Contribute"},
		{"/api.html", "Endpoints"},
		{"/api2.html", "<rapi-doc"},
		{"/api.json", `"consumes"`},
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

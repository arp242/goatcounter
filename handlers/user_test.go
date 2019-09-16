package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/zhttp"
)

func TestUserNew(t *testing.T) {
	tests := []handlerTest{
		{
			name:         "basic",
			router:       NewBackend,
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
			router:       NewBackend,
			method:       "POST",
			path:         "/user/requestlogin",
			body:         map[string]string{"email": "new@example.com"},
			wantCode:     303,
			wantFormCode: 303,
		},
		{
			name:         "nonexistent",
			router:       NewBackend,
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
				want = `&{i All good. Login URL emailed to "new@example.com"; please click it in the next 15 minutes to continue.`
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

				_, err = goatcounter.MustGetDB(ctx).ExecContext(ctx, `update users set
					login_request='asdf', login_at=current_timestamp
					where id=$2 and site=1`, user.ID)
				if err != nil {
					panic(err)
				}
			},
			router:       NewBackend,
			path:         "/user/login/asdf",
			wantCode:     303,
			wantFormCode: 303,
		},

		{
			name:         "nonexistent",
			router:       NewBackend,
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
			router:       NewBackend,
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

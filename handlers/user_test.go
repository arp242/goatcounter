package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestUserTpl(t *testing.T) {
	tests := []handlerTest{
		{
			name:   "user_new",
			router: newBackend,
			setup: func(ctx context.Context, t *testing.T) {
				u := goatcounter.User{
					Site:     1,
					Email:    "user_test@example.com",
					Password: []byte("coconuts"),
					Access:   goatcounter.UserAccesses{"all": goatcounter.AccessAdmin},
				}
				err := u.Insert(ctx, false)
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
	ctx := gctest.DB(t)

	r, rr := newTest(ctx, "POST", "/user/requestlogin", nil)
	body, ct, err := ztest.MultipartForm(map[string]string{
		"email":    "test@gctest.localhost",
		"password": "coconuts",
	})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", ct)
	r.Body = io.NopCloser(body)

	r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
	newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
	ztest.Code(t, rr, 303)

	if f := zhttp.ReadFlash(rr, r); f != nil {
		t.Errorf("FLASH AHAAAAA! %#v\n", f)
	}
	if l := rr.Header().Get("Location"); l != "/" {
		t.Error(l)
	}
	if c := rr.Header().Get("Set-Cookie"); !strings.HasPrefix(c, "key="+ztime.Now(ctx).Format("20060102")+"-") {
		t.Error(c)
	}
}

func TestUserForgot(t *testing.T) {
	ctx := gctest.DB(t)

	{ // Load form.
		r, rr := newTest(ctx, "GET", "/user/forgot", nil)
		r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
		if !strings.Contains(rr.Body.String(), "Forgot password") {
			t.Error(rr.Body.String())
		}
	}

	{ // Submit form
		r, rr := newTest(ctx, "POST", "/user/request-reset", nil)
		body, ct, err := ztest.MultipartForm(map[string]string{
			"email": "test@gctest.localhost",
		})
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", ct)
		r.Body = io.NopCloser(body)

		r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 303)
		f := zhttp.ReadFlash(rr, r)
		if f == nil {
			t.Error("f == nil")
		}

		want := `Email sent to test@gctest.localhost`
		if f != nil && f.Message != want {
			t.Errorf("flash wrong:\nwant: %q\nhave: %q", want, f.Message)
		}
	}

	{ // Load reset page.
		err := User(ctx).ByID(ctx, 1) // Reload token from DB.
		if err != nil {
			t.Fatal(err)
		}

		r, rr := newTest(ctx, "GET", "/user/reset/"+*User(ctx).LoginRequest, nil)
		r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 200)
		if !strings.Contains(rr.Body.String(), "New password") {
			t.Error(rr.Body.String())
		}
	}

	{ // Submit reset page
		r, rr := newTest(ctx, "POST", "/user/reset/"+*User(ctx).LoginRequest, nil)
		body, ct, err := ztest.MultipartForm(map[string]string{
			"password":  "grapefruit",
			"password2": "grapefruit",
		})
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", ct)
		r.Body = io.NopCloser(body)

		r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
		newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
		ztest.Code(t, rr, 303)
		f := zhttp.ReadFlash(rr, r)
		if f == nil {
			t.Error("f == nil")
		}
		if f != nil && f.Message != `Password reset; use your new password to login.` {
			t.Error(f)
		}
	}
}

func TestUserLoginMFA(t *testing.T) {
	ctx := gctest.DB(t)

	user := User(ctx)
	err := user.EnableTOTP(ctx)
	if err != nil {
		t.Fatal(err)
	}

	r, rr := newTest(ctx, "POST", "/user/requestlogin", nil)
	body, ct, err := ztest.MultipartForm(map[string]string{
		"email":    "test@gctest.localhost",
		"password": "coconuts",
	})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", ct)
	r.Body = io.NopCloser(body)

	r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
	newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
	ztest.Code(t, rr, 200)

	var mac, logintoken string
	{
		m := regexp.MustCompile(`<input type="hidden" name="loginmac" value="([^"]+)">`).
			FindStringSubmatch(rr.Body.String())
		if len(m) != 2 {
			t.Fatal()
		}
		if m[1] == "" {
			t.Fatal()
		}
		mac = m[1]
	}
	{
		m := regexp.MustCompile(`<input type="hidden" name="user_logintoken" value="([^"]+)">`).
			FindStringSubmatch(rr.Body.String())
		if len(m) != 2 {
			t.Fatal()
		}
		if m[1] == "" {
			t.Fatal()
		}
		logintoken = m[1]
	}

	testTOTP = true
	defer func() { testTOTP = false }()

	r, rr = newTest(ctx, "POST", "/user/totplogin", nil)
	body, ct, err = ztest.MultipartForm(map[string]string{
		"loginmac":        mac,
		"user_logintoken": logintoken,
		"totp_token":      "123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", ct)
	r.Body = io.NopCloser(body)

	r.Host = Site(ctx).Code + "." + goatcounter.Config(ctx).Domain
	newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
	ztest.Code(t, rr, 303)
	if f := zhttp.ReadFlash(rr, r); f != nil {
		t.Errorf("FLASH AHAAAAA! %#v\n", f)
	}
	if l := rr.Header().Get("Location"); l != "/" {
		t.Error(l)
	}
	if c := rr.Header().Get("Set-Cookie"); !strings.HasPrefix(c, "key="+ztime.Now(ctx).Format("20060102")+"-") {
		t.Error(c)
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

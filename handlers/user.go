// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"golang.org/x/crypto/bcrypt"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

type user struct{}

func (h user) mount(r chi.Router) {
	r.Get("/user/new", zhttp.Wrap(h.new))
	r.Get("/user/forgot", zhttp.Wrap(h.forgot))
	r.Post("/user/request-reset", zhttp.Wrap(h.requestReset))

	// Rate limit login attempts.
	rate := r.With(zhttp.Ratelimit(zhttp.RatelimitOptions{
		Client: zhttp.RatelimitIP,
		Store:  zhttp.NewRatelimitMemory(),
		Limit:  zhttp.RatelimitLimit(20, 60),
	}))
	rate.Post("/user/requestlogin", zhttp.Wrap(h.requestLogin))
	rate.Get("/user/login/{key}", zhttp.Wrap(h.login))
	rate.Get("/user/reset/{key}", zhttp.Wrap(h.reset))
	rate.Get("/user/verify/{key}", zhttp.Wrap(h.verify))
	rate.Post("/user/reset/{key}", zhttp.Wrap(h.doReset))

	auth := r.With(loggedIn)
	auth.Post("/user/logout", zhttp.Wrap(h.logout))
	auth.Post("/user/change-password", zhttp.Wrap(h.changePassword))
	auth.Post("/user/resend-verify", zhttp.Wrap(h.resendVerify))
}

func (h user) new(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	if u != nil && u.ID > 0 {
		return zhttp.SeeOther(w, "/")
	}

	var user goatcounter.User
	err := user.BySite(r.Context(), goatcounter.MustGetSite(r.Context()).IDOrParent())
	if err != nil {
		return err
	}
	haspw := len(user.Password) > 0

	return zhttp.Template(w, "user.gohtml", struct {
		Globals
		Email       string
		HasPassword bool
	}{newGlobals(w, r), r.URL.Query().Get("email"), haspw})
}

func (h user) forgot(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	if u != nil && u.ID > 0 {
		return zhttp.SeeOther(w, "/")
	}

	return zhttp.Template(w, "user_forgot_pw.gohtml", struct {
		Globals
		Email    string
		Page     string
		MetaDesc string
	}{newGlobals(w, r), r.URL.Query().Get("email"), "forgot_pw", "Forgot password – GoatCounter"})
}

func (h user) requestReset(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	if u != nil && u.ID > 0 {
		return zhttp.SeeOther(w, "/")
	}

	// Legacy email flow.
	args := struct {
		Email string `json:"email"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.MustGetSite(r.Context())
	var user goatcounter.User
	err = user.BySite(r.Context(), site.IDOrParent())
	if err != nil {
		return err
	}

	if !strings.EqualFold(args.Email, user.Email) {
		zhttp.FlashError(w, "Unknown email: %q", args.Email)
		return zhttp.SeeOther(w, "/user/forgot")
	}

	err = u.ByEmail(r.Context(), args.Email)
	if err != nil {
		if zdb.ErrNoRows(err) {
			zhttp.FlashError(w, "Not an account on this site: %q", args.Email)
			return zhttp.SeeOther(w, fmt.Sprintf("/user/new?email=%s", url.QueryEscape(args.Email)))
		}
		return err
	}

	err = u.RequestReset(r.Context())
	if err != nil {
		return err
	}

	go func() {
		defer zlog.Recover()
		err = zmail.SendTemplate(
			fmt.Sprintf("Password reset for %s", site.Domain()),
			mail.Address{Name: "GoatCounter login", Address: cfg.LoginFrom},
			[]mail.Address{{Name: u.Name, Address: u.Email}},
			"email_password_reset.gotxt", struct {
				Site goatcounter.Site
				User goatcounter.User
			}{*site, *u})
		if err != nil {
			zlog.Errorf("password reset: %s", err)
		}
	}()

	zhttp.Flash(w, "Email sent to %q", args.Email)
	return zhttp.SeeOther(w, "/user/forgot")
}

func (h user) requestLogin(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	if u != nil && u.ID > 0 {
		return zhttp.SeeOther(w, "/")
	}

	site := goatcounter.MustGetSite(r.Context())

	var user goatcounter.User
	err := user.BySite(r.Context(), site.IDOrParent())
	if err != nil {
		return err
	}

	// Password flow.
	if len(user.Password) > 0 {
		args := struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}{}
		_, err := zhttp.Decode(r, &args)
		if err != nil {
			return err
		}

		if !strings.EqualFold(args.Email, user.Email) {
			zhttp.FlashError(w, "Wrong password for %q", args.Email)
			return zhttp.SeeOther(w, "/user/new")
		}

		err = bcrypt.CompareHashAndPassword(user.Password, []byte(args.Password))
		if err != nil {
			if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
				zhttp.FlashError(w, "Wrong password for %q", args.Email)
			}
			return zhttp.SeeOther(w, "/user/new")
		}

		err = user.Login(r.Context())
		if err != nil {
			return err
		}

		zhttp.SetCookie(w, *user.LoginToken, site.Domain())
		return zhttp.SeeOther(w, "/")
	}

	// Legacy email flow.
	args := struct {
		Email string `json:"email"`
	}{}
	_, err = zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	err = u.ByEmail(r.Context(), args.Email)
	if err != nil {
		if zdb.ErrNoRows(err) {
			zhttp.FlashError(w, "Not an account on this site: %q", args.Email)
			return zhttp.SeeOther(w, fmt.Sprintf("/user/new?email=%s", url.QueryEscape(args.Email)))
		}

		return err
	}

	err = u.RequestLogin(r.Context())
	if err != nil {
		return err
	}

	go u.SendLoginMail(context.Background(), goatcounter.MustGetSite(r.Context()))
	flashLoginKey(r.Context(), w, u.Email)
	return zhttp.SeeOther(w, "/user/new")
}

func (h user) login(w http.ResponseWriter, r *http.Request) error {
	var u goatcounter.User
	err := u.ByLoginRequest(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		if !zdb.ErrNoRows(err) {
			zlog.Error(err)
		}
		return guru.New(http.StatusForbidden, "could not login; perhaps the key has expired?")
	}

	err = u.Login(r.Context())
	if err != nil {
		return err
	}

	// Temporary Set-Cookie to remove previous "code.goatcounter.com" cookie,
	// which takes priority over the ".goatcounter.com" cookie.
	http.SetCookie(w, &http.Cookie{
		Name:    "key",
		Value:   "",
		Path:    "/",
		Expires: goatcounter.Now().Add(-100 * time.Hour),
	})

	zhttp.SetCookie(w, *u.LoginToken, goatcounter.MustGetSite(r.Context()).Domain())
	return zhttp.SeeOther(w, "/")
}

func (h user) reset(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())
	key := chi.URLParam(r, "key")

	var user goatcounter.User
	err := user.ByResetToken(r.Context(), key)
	if err != nil {
		if !zdb.ErrNoRows(err) {
			zlog.Error(err)
		}
		return guru.New(http.StatusForbidden, "could find the user for the given token; perhaps it's expired or has already been used?")
	}

	user.ID = 0 // Don't count as logged in.

	return zhttp.Template(w, "user_reset.gohtml", struct {
		Globals
		Site *goatcounter.Site
		User goatcounter.User
		Key  string
	}{newGlobals(w, r), site, user, key})
}

func (h user) doReset(w http.ResponseWriter, r *http.Request) error {
	var user goatcounter.User
	err := user.ByResetToken(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		return guru.New(http.StatusForbidden, "could find the user for the given token; perhaps it's expired or has already been used?")
	}

	var args struct {
		Password  string `json:"password"`
		Password2 string `json:"password2"`
	}
	_, err = zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	if args.Password != args.Password2 {
		zhttp.FlashError(w, "Password confirmation doesn’t match.")
		return zhttp.SeeOther(w, "/user/new")
	}

	err = user.UpdatePassword(r.Context(), args.Password)
	if err != nil {
		var vErr *zvalidate.Validator
		if errors.As(err, &vErr) {
			zhttp.FlashError(w, fmt.Sprintf("%s", err))
			return zhttp.SeeOther(w, "/user/new")
		}
		return err
	}

	zhttp.Flash(w, "Password reset; use your new password to login.")
	return zhttp.SeeOther(w, "/user/new")
}

func (h user) logout(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	err := u.Logout(r.Context())
	if err != nil {
		zlog.Errorf("logout: %s", err)
	}

	zhttp.ClearCookie(w, goatcounter.MustGetSite(r.Context()).Domain())
	return zhttp.SeeOther(w, "/")
}

func (h user) changePassword(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	var args struct {
		CPassword string `json:"c_password"`
		Password  string `json:"password"`
		Password2 string `json:"password2"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	if len(u.Password) > 0 {
		ok, err := u.CorrectPassword(args.CPassword)
		if err != nil {
			return err
		}
		if !ok {
			zhttp.FlashError(w, "Current password is incorrect.")
			return zhttp.SeeOther(w, "/settings#tab-change-password")
		}
	}

	if args.Password != args.Password2 {
		zhttp.FlashError(w, "Password confirmation doesn’t match.")
		return zhttp.SeeOther(w, "/settings#tab-change-password")
	}

	err = u.UpdatePassword(r.Context(), args.Password)
	if err != nil {
		var vErr *zvalidate.Validator
		if errors.As(err, &vErr) {
			zhttp.FlashError(w, fmt.Sprintf("%s", err))
			return zhttp.SeeOther(w, "/settings#tab-change-password")
		}
		return err
	}

	zhttp.Flash(w, "Password changed")
	return zhttp.SeeOther(w, "/")
}

func (h user) resendVerify(w http.ResponseWriter, r *http.Request) error {
	user := goatcounter.GetUser(r.Context())
	if user.EmailVerified {
		zhttp.Flash(w, "%q is already verified", user.Email)
		return zhttp.SeeOther(w, "/")
	}

	site := goatcounter.MustGetSite(r.Context())

	sendEmailVerify(site, user)

	zhttp.Flash(w, "Sent to %q", user.Email)
	return zhttp.SeeOther(w, "/")
}

func sendEmailVerify(site *goatcounter.Site, user *goatcounter.User) {
	go func() {
		defer zlog.Recover()
		err := zmail.SendTemplate("Verify your email",
			mail.Address{Name: "GoatCounter", Address: cfg.LoginFrom},
			[]mail.Address{{Name: user.Name, Address: user.Email}},
			"email_verify.gotxt", struct {
				Site goatcounter.Site
				User goatcounter.User
			}{*site, *user})
		if err != nil {
			zlog.Errorf("zmail: %s", err)
		}
	}()
}

func (h user) verify(w http.ResponseWriter, r *http.Request) error {
	user := goatcounter.GetUser(r.Context())
	if user.EmailVerified {
		zhttp.Flash(w, "%q is already verified", user.Email)
		return zhttp.SeeOther(w, "/")
	}

	key := chi.URLParam(r, "key")
	if key != *user.EmailToken {
		zhttp.FlashError(w, "Wrong verification key")
		return zhttp.SeeOther(w, "/")
	}

	err := user.VerifyEmail(r.Context())
	if err != nil {
		return err
	}

	zhttp.Flash(w, "%q verified", user.Email)
	return zhttp.SeeOther(w, "/")
}

func flashLoginKey(ctx context.Context, w http.ResponseWriter, email string) {
	msg := fmt.Sprintf(
		"All good. Login URL emailed to %q; please click it in the next hour to continue.",
		email)

	if !cfg.Prod {
		site := goatcounter.MustGetSite(ctx)
		var u goatcounter.User
		err := u.ByEmail(ctx, email)
		if err != nil {
			zlog.Error(err)
		} else {
			url := fmt.Sprintf("%s/user/login/%s", site.URL(), *u.LoginRequest)
			msg += fmt.Sprintf(
				"<br>\n<small>URL on dev for convenience: <a href='%s'>%[1]s</a></small>",
				url)
		}
	}

	zhttp.Flash(w, msg)
}

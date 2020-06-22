// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.soquee.net/otp"
	"github.com/go-chi/chi"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/xsrftoken"
	"zgo.at/blackmail"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/goatcounter/cfg"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const (
	actionTOTP = "totp"
	mfaError   = "Token did not match; perhaps you waited too long? Try again."
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
	rate.Post("/user/totplogin", zhttp.Wrap(h.totpLogin))
	rate.Get("/user/reset/{key}", zhttp.Wrap(h.reset))
	rate.Get("/user/verify/{key}", zhttp.Wrap(h.verify))
	rate.Post("/user/reset/{key}", zhttp.Wrap(h.doReset))

	auth := r.With(loggedIn)
	auth.Post("/user/logout", zhttp.Wrap(h.logout))
	auth.Post("/user/change-password", zhttp.Wrap(h.changePassword))
	auth.Post("/user/disable-totp", zhttp.Wrap(h.disableTOTP))
	auth.Post("/user/enable-totp", zhttp.Wrap(h.enableTOTP))
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

	return zhttp.Template(w, "user.gohtml", struct {
		Globals
		Email string
	}{newGlobals(w, r), r.URL.Query().Get("email")})
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

	bgrun.Run(func() {
		err := blackmail.Send(
			fmt.Sprintf("Password reset for %s", site.Domain()),
			blackmail.From("GoatCounter login", cfg.EmailFrom),
			blackmail.To(u.Email),
			blackmail.BodyMustText(goatcounter.EmailTemplate("email_password_reset.gotxt", struct {
				Site goatcounter.Site
				User goatcounter.User
			}{*site, *u})))
		if err != nil {
			zlog.Errorf("password reset: %s", err)
		}
	})

	zhttp.Flash(w, "Email sent to %q", args.Email)
	return zhttp.SeeOther(w, "/user/forgot")
}

func (h user) totpLogin(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		LoginMAC string `json:"loginmac"`
		Token    string `json:"totp_token"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.MustGetSite(r.Context())

	var u goatcounter.User
	err = u.BySite(r.Context(), site.IDOrParent())
	if err != nil {
		return err
	}

	valid := xsrftoken.Valid(args.LoginMAC, *u.LoginToken, strconv.FormatInt(u.ID, 10), actionTOTP)
	if !valid {
		zhttp.Flash(w, "Invalid login")
		return zhttp.SeeOther(w, "/")
	}

	tokGen := otp.NewOTP(u.TOTPSecret, 6, sha1.New, otp.TOTP(30*time.Second, time.Now))
	tokInt, err := strconv.ParseInt(args.Token, 10, 32)
	if err != nil {
		return err
	}

	// Check a 30 second window on either side of the current time as well. It's
	// common for clocks to be slightly out of sync and this prevents most errors
	// and is what the spec recommends.
	if tokGen(0, nil) != int32(tokInt) &&
		tokGen(-1, nil) != int32(tokInt) &&
		tokGen(1, nil) != int32(tokInt) {
		zhttp.FlashError(w, mfaError)
		return zhttp.Template(w, "totp.gohtml", struct {
			Globals
			LoginMAC string
		}{newGlobals(w, r), args.LoginMAC})
	}

	zhttp.SetCookie(w, *u.LoginToken, site.Domain())
	return zhttp.SeeOther(w, "/")
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

	args := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}
	_, err = zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	if !strings.EqualFold(args.Email, user.Email) {
		zhttp.FlashError(w, "Wrong password for %q", args.Email)
		return zhttp.SeeOther(w, "/user/new")
	}

	if user.Password == nil {
		zhttp.FlashError(w,
			"There is no password set for %q; please use <a href='/user/forgot?email=%[1]s'>Forgot password</a> to set it.",
			args.Email)
		return zhttp.SeeOther(w, "/user/new?email="+url.QueryEscape(args.Email))
	}

	err = bcrypt.CompareHashAndPassword(user.Password, []byte(args.Password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			zhttp.FlashError(w, "Wrong password for %q", args.Email)
		} else {
			zhttp.FlashError(w, "Something went wrong :-( An error has been logged for investigation.")
			zlog.Error(err)
		}
		return zhttp.SeeOther(w, "/user/new?email="+url.QueryEscape(args.Email))
	}

	err = user.Login(r.Context())
	if err != nil {
		return err
	}

	if user.TOTPEnabled {
		return zhttp.Template(w, "totp.gohtml", struct {
			Globals
			LoginMAC string
		}{newGlobals(w, r), xsrftoken.Generate(*user.LoginToken, strconv.FormatInt(user.ID, 10), actionTOTP)})
	}

	zhttp.SetCookie(w, *user.LoginToken, site.Domain())
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

func (h user) disableTOTP(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	err := u.DisableTOTP(r.Context())
	if err != nil {
		return err
	}

	return zhttp.SeeOther(w, "/settings#tab-auth")
}

func (h user) enableTOTP(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	var args struct {
		Token string `json:"totp_token"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	tokGen := otp.NewOTP(u.TOTPSecret, 6, sha1.New, otp.TOTP(30*time.Second, time.Now))
	tokInt, err := strconv.ParseInt(args.Token, 10, 32)
	if err != nil {
		return err
	}

	// Check a 30 second window on either side of the current time as well. It's
	// common for clocks to be slightly out of sync and this prevents most errors
	// and is what the spec recommends.
	if tokGen(0, nil) != int32(tokInt) &&
		tokGen(-1, nil) != int32(tokInt) &&
		tokGen(1, nil) != int32(tokInt) {
		zhttp.FlashError(w, mfaError)
		return zhttp.SeeOther(w, "/settings#tab-auth")
	}

	err = u.EnableTOTP(r.Context())
	if err != nil {
		return err
	}
	return zhttp.SeeOther(w, "/settings#tab-auth")
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

	sendEmailVerify(goatcounter.MustGetSite(r.Context()), user)
	zhttp.Flash(w, "Sent to %q", user.Email)
	return zhttp.SeeOther(w, "/")
}

func sendEmailVerify(site *goatcounter.Site, user *goatcounter.User) {
	bgrun.Run(func() {
		err := blackmail.Send("Verify your email",
			mail.Address{Name: "GoatCounter", Address: cfg.EmailFrom},
			blackmail.To(user.Email),
			blackmail.BodyMustText(goatcounter.EmailTemplate("email_verify.gotxt", struct {
				Site goatcounter.Site
				User goatcounter.User
			}{*site, *user})))
		if err != nil {
			zlog.Errorf("blackmail: %s", err)
		}
	})
}

func (h user) verify(w http.ResponseWriter, r *http.Request) error {
	key := chi.URLParam(r, "key")
	var user goatcounter.User
	err := user.ByEmailToken(r.Context(), key)
	if err != nil {
		if zdb.ErrNoRows(err) {
			return guru.New(400, "unknown token; perhaps it was already used?")
		}
		return err
	}

	if user.EmailVerified {
		zhttp.Flash(w, "%q is already verified", user.Email)
		return zhttp.SeeOther(w, "/")
	}

	if key != *user.EmailToken {
		zhttp.FlashError(w, "Wrong verification key")
		return zhttp.SeeOther(w, "/")
	}

	err = user.VerifyEmail(r.Context())
	if err != nil {
		return err
	}

	zhttp.Flash(w, "%q verified", user.Email)
	return zhttp.SeeOther(w, "/")
}

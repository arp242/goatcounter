// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
)

type user struct{}

func (h user) mount(r chi.Router) {
	r.Get("/user/new", zhttp.Wrap(h.new))

	// Rate limit login attempts.
	rate := r.With(zhttp.Ratelimit(zhttp.RatelimitIP, zhttp.NewRatelimitMemory(), 20, 60))
	rate.Post("/user/requestlogin", zhttp.Wrap(h.requestLogin))
	rate.Get("/user/login/{key}", zhttp.Wrap(h.login))

	auth := r.With(loggedIn)
	auth.Post("/user/logout", zhttp.Wrap(h.logout))
}

func (h user) new(w http.ResponseWriter, r *http.Request) error {
	// During login we can't set the flash cookie as the domain is different, so
	// pass it by query.
	if m := r.URL.Query().Get("mailed"); m != "" {
		flashLoginKey(r.Context(), w, m)
	}

	return zhttp.Template(w, "user.gohtml", struct {
		Globals
		Email string
	}{newGlobals(w, r), r.URL.Query().Get("email")})
}

func (h user) requestLogin(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		Email string `json:"email"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	var u goatcounter.User
	err = u.ByEmail(r.Context(), args.Email)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			zhttp.FlashError(w, "Not an account on this site: %q", args.Email)
			return zhttp.SeeOther(w, fmt.Sprintf("/user/new?email=%s", url.QueryEscape(args.Email)))
		}

		return err
	}

	err = u.RequestLogin(r.Context())
	if err != nil {
		return err
	}

	var site goatcounter.Site
	err = site.ByID(r.Context(), u.Site)
	if err != nil {
		return err
	}

	go u.SendLoginMail(context.Background(), site)
	flashLoginKey(r.Context(), w, u.Email)
	return zhttp.SeeOther(w, "/user/new")
}

func (h user) login(w http.ResponseWriter, r *http.Request) error {
	var u goatcounter.User
	err := u.ByKey(r.Context(), chi.URLParam(r, "key"))
	if err != nil {
		if errors.Cause(err) != sql.ErrNoRows {
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
		Expires: time.Now().Add(-100 * time.Hour),
	})

	zhttp.SetCookie(w, *u.Key.LoginKey, cfg.Domain)
	zhttp.Flash(w, "Welcome %s", u.Name)
	return zhttp.SeeOther(w, "/")
}

func (h user) logout(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	err := u.Logout(r.Context())
	if err != nil {
		zlog.Errorf("logout: %s", err)
	}

	zhttp.ClearCookie(w)
	zhttp.Flash(w, "&#x1f44b;")
	return zhttp.SeeOther(w, "/")
}

func flashLoginKey(ctx context.Context, w http.ResponseWriter, email string) {
	msg := fmt.Sprintf(
		"All good. Login URL emailed to %q; please click it in the next 15 minutes to continue.",
		email)

	if !cfg.Prod {
		site := goatcounter.MustGetSite(ctx)
		var key *string
		err := goatcounter.MustGetDB(ctx).GetContext(ctx, &key, `
			select login_key from user_keys
			join users on users.id=user_keys.user
			where user_keys.site=$1 and lower(users.email)=lower($2) and login_req is not null
			order by login_req desc limit 1`, site.ID, email)
		if err != nil {
			zlog.Error(err)
		} else {
			url := fmt.Sprintf("%s.%s/user/login/%s", site.Code, cfg.Domain, *key)
			msg += fmt.Sprintf(
				"<br>\n<small>URL on dev for convenience: <a href='//%s'>%[1]s</a></small>",
				url)
		}
	}

	zhttp.Flash(w, msg)
}

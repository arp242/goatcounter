// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/mail"

	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/smail"
)

type user struct{}

func (h user) mount(r chi.Router) {
	// Rate limit login attempts.
	rate := zhttp.Ratelimit(zhttp.RatelimitIP, zhttp.NewRatelimitMemory(), 20, 60)

	r.Get("/user/new", zhttp.Wrap(h.new))
	r.With(rate).Post("/user/requestlogin", zhttp.Wrap(h.requestLogin))
	r.With(rate).Get("/user/login/{key}", zhttp.Wrap(h.login))
	a := r.With(filterLoggedIn)
	a.Post("/user/logout", zhttp.Wrap(h.logout))
	//a.Post("/user/save", zhttp.Wrap(h.save))
}

func (h user) new(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "user.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
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
		if errors.Cause(err) != sql.ErrNoRows {
			zlog.Error(err)
		}
		return guru.New(http.StatusForbidden, "Can't log you in. Sorry :-(")
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

	var url = fmt.Sprintf("%s.%s/user/login/%s", site.Code, cfg.Domain, *u.LoginKey)
	go func() {
		err := smail.Send("Your login URL",
			mail.Address{Name: "GoatCounter login", Address: "login@goatcounter.com"},
			[]mail.Address{{Name: u.Name, Address: u.Email}},
			fmt.Sprintf("Hi there,\n\nYour login URL for Goatcounter is:\n\n  https://%s\n\nGo to it to log in.\n",
				url))
		if err != nil {
			zlog.Errorf("smail: %s", err)
		}
	}()

	if cfg.Prod {
		zhttp.Flash(w,
			"All good. Login URL emailed to %q; please click it in the next 15 minutes to continue.",
			u.Email)
	} else {
		// Show URL on dev for convenience.
		zhttp.Flash(w, "<a href='http://%s'>http://%[1]s</a>", url)
	}
	return zhttp.SeeOther(w, "/")
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

	zhttp.SetCookie(w, *u.LoginKey)
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

func (h user) save(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())
	_, err := zhttp.Decode(r, u)
	if err != nil {
		return err
	}

	fmt.Println(u)

	return zhttp.SeeOther(w, "/")
}

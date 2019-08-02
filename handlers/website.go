// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers // import "zgo.at/goatcounter/handlers"

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"github.com/teamwork/validate"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zhttp"
)

type Website struct{}

func (h Website) Mount(r *chi.Mux, db *sqlx.DB) {
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		middleware.RedirectSlashes,
		addctx(db, false),
		zhttp.Headers(nil),
		zhttp.Log(true, ""),
		keyAuth)

	r.Get("/status", zhttp.Wrap(h.status()))
	r.Get("/signup/{plan}", zhttp.Wrap(h.signup))
	r.Post("/signup/{plan}", zhttp.Wrap(h.doSignup))
	for _, t := range []string{"", "help", "privacy", "terms"} {
		r.Get("/"+t, zhttp.Wrap(h.tpl))
	}
	user{}.mount(r)
}

func (h Website) tpl(w http.ResponseWriter, r *http.Request) error {
	t := r.URL.Path[1:]
	if t == "" {
		t = "home"
	}
	return zhttp.Template(w, t+".gohtml", struct {
		Globals
		Page string
	}{newGlobals(w, r), t})
}

func (h Website) status() func(w http.ResponseWriter, r *http.Request) error {
	started := time.Now()
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.JSON(w, map[string]string{
			"uptime":  time.Now().Sub(started).String(),
			"version": cfg.Version,
		})
	}
}

func (h Website) signup(w http.ResponseWriter, r *http.Request) error {
	plan, planName, err := getPlan(r)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "signup.gohtml", struct {
		Globals
		Page     string
		Plan     string
		PlanName string
		Site     goatcounter.Site
		User     goatcounter.User
		Validate map[string][]string
	}{newGlobals(w, r), "signup", plan, planName, goatcounter.Site{},
		goatcounter.User{}, map[string][]string{}})
}

func (h Website) doSignup(w http.ResponseWriter, r *http.Request) error {
	if cfg.Prod { // TODO(public)
		return errors.New("signups not on production yet")
	}

	plan, planName, err := getPlan(r)
	if err != nil {
		return err
	}

	args := struct {
		Domain string `json:"site_domain"`
		Code   string `json:"site_code"`
		Email  string `json:"user_email"`
		Name   string `json:"user_name"`
	}{}
	_, err = zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	v := validate.New()

	// Create site.
	site := goatcounter.Site{
		Domain: args.Domain,
		Code:   args.Code,
		Plan:   plan,
	}
	site.Defaults(r.Context())
	err = site.Validate(r.Context())
	if err != nil {
		if _, ok := err.(*validate.Validator); !ok {
			return err
		}
		v.Sub("site", "", err)
	}

	// Create user.
	user := goatcounter.User{
		Name:  args.Name,
		Email: args.Email,
	}
	user.Defaults(r.Context())
	err = user.Validate(r.Context())
	if err != nil {
		if _, ok := err.(*validate.Validator); !ok {
			return err
		}
		v.Sub("user", "", err)
	}

	if v.HasErrors() {
		return zhttp.Template(w, "signup.gohtml", struct {
			Globals
			Page     string
			Plan     string
			PlanName string
			Site     goatcounter.Site
			User     goatcounter.User
			Validate map[string][]string
		}{newGlobals(w, r), "signup", plan, planName, site, user, v.Errors})
	}

	// Insert data.
	err = site.Insert(r.Context())
	if err != nil {
		return err
	}
	user.Site = site.ID
	err = user.Insert(r.Context())
	if err != nil {
		return err
	}

	return zhttp.SeeOther(w, "/signup/"+plan)
}

func getPlan(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "plan")
	switch name {
	case "personal":
		return "p", name, nil
	case "business":
		return "b", name, nil
	case "enterprise":
		return "e", name, nil
	default:
		return "", name, guru.Errorf(400, "unknown plan: %q", name)
	}
}

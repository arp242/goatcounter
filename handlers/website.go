// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers // import "zgo.at/goatcounter/handlers"

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zvalidate"
)

type website struct{}

func (h website) Mount(r *chi.Mux, db *sqlx.DB) {
	// TODO: Frequent 404s I should probably fix:
	//       /robots.txt
	//       /ads.txt               https://en.wikipedia.org/wiki/Ads.txt
	//       /security.txt          https://en.wikipedia.org/wiki/Security.txt
	//       /apple-touch-icon.png
	//       /sitemap.xml

	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		middleware.RedirectSlashes,
		addctx(db, false),
		zhttp.Headers(nil))
	if !cfg.Prod {
		zhttp.Log(true, "")
	}

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, 404, errors.New("Not Found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, 405, errors.New("Method Not Allowed"))
	})

	r.Get("/status", zhttp.Wrap(h.status()))
	r.Get("/signup/{plan}", zhttp.Wrap(h.signup))
	r.Post("/signup/{plan}", zhttp.Wrap(h.doSignup))
	for _, t := range []string{"", "help", "privacy", "terms"} {
		r.Get("/"+t, zhttp.Wrap(h.tpl))
	}
	user{}.mount(r)
}

func (h website) tpl(w http.ResponseWriter, r *http.Request) error {
	t := r.URL.Path[1:]
	if t == "" {
		t = "home"
	}
	return zhttp.Template(w, t+".gohtml", struct {
		Globals
		Page string
	}{newGlobals(w, r), t})
}

func (h website) status() func(w http.ResponseWriter, r *http.Request) error {
	started := time.Now().UTC()
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.JSON(w, map[string]string{
			"uptime":  time.Now().UTC().Sub(started).String(),
			"version": cfg.Version,
		})
	}
}

func (h website) signup(w http.ResponseWriter, r *http.Request) error {
	plan, planName, err := getPlan(r)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "signup.gohtml", struct {
		Globals
		Page       string
		Plan       string
		PlanName   string
		Site       goatcounter.Site
		User       goatcounter.User
		Validate   map[string][]string
		TuringTest string
	}{newGlobals(w, r), "signup", plan, planName, goatcounter.Site{},
		goatcounter.User{}, map[string][]string{}, ""})
}

type signupArgs struct {
	Name       string `json:"site_name"`
	Code       string `json:"site_code"`
	Email      string `json:"user_email"`
	UserName   string `json:"user_name"`
	TuringTest string `json:"turing_test"`
}

func (h website) doSignup(w http.ResponseWriter, r *http.Request) error {
	plan, planName, err := getPlan(r)
	if err != nil {
		return err
	}

	var args signupArgs
	_, err = zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	txctx, tx, err := zdb.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	v := zvalidate.New()
	if strings.TrimSpace(args.TuringTest) != "9" {
		v.Append("turing_test", "must fill in correct value")
	}

	// Create site.
	site := goatcounter.Site{
		Name: args.Name,
		Code: args.Code,
		Plan: plan,
	}
	err = site.Insert(txctx)
	if err != nil {
		if _, ok := err.(*zvalidate.Validator); !ok {
			return err
		}
		v.Sub("site", "", err)
	}

	// Create user.
	user := goatcounter.User{
		Name:  args.Name,
		Email: args.Email,
		Site:  site.ID,
	}
	err = user.Insert(txctx)
	if err != nil {
		if _, ok := err.(*zvalidate.Validator); !ok {
			return err
		}
		v.Sub("user", "", err)
	}

	if v.HasErrors() {
		return zhttp.Template(w, "signup.gohtml", struct {
			Globals
			Page       string
			Plan       string
			PlanName   string
			Site       goatcounter.Site
			User       goatcounter.User
			Validate   map[string][]string
			TuringTest string
		}{newGlobals(w, r), "signup", plan, planName, site, user, v.Errors, args.TuringTest})
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// Login
	ctx := context.WithValue(r.Context(), ctxkey.Site, &site)
	err = user.RequestLogin(ctx)
	if err != nil {
		return err
	}
	go user.SendLoginMail(context.Background(), &site)

	return zhttp.SeeOther(w, fmt.Sprintf("%s/user/new?mailed=%s",
		site.URL(), url.QueryEscape(user.Email)))
}

func getPlan(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "plan")
	switch name {
	case "personal":
		return "p", name, nil
	case "business":
		return "b", name, nil
	//case "enterprise":
	//	return "e", name, nil
	default:
		return "", name, guru.Errorf(400, "unknown plan: %q", name)
	}
}

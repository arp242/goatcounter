// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers // import "zgo.at/goatcounter/handlers"

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/errors"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

type website struct{}

func (h website) Mount(r *chi.Mux, db zdb.DB) {
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

	r.Get("/robots.txt", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Text(w, "")
	}))
	r.Get("/ads.txt", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Text(w, "")
	}))
	r.Get("/security.txt", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.Text(w, "Contact: support@goatcounter.com")
	}))

	r.Get("/status", zhttp.Wrap(h.status()))
	r.Get("/signup", zhttp.Wrap(h.signup))
	r.Post("/signup", zhttp.Wrap(h.doSignup))
	r.Get("/user/forgot", zhttp.Wrap(h.forgot))
	r.Post("/user/forgot", zhttp.Wrap(h.doForgot))
	for _, t := range []string{"", "help", "privacy", "terms", "contact", "contribute"} {
		r.Get("/"+t, zhttp.Wrap(h.tpl))
	}
}

var metaDesc = map[string]string{
	"":           "Simple web statistics. No tracking of personal data.",
	"help":       "Help and support – GoatCounter",
	"privacy":    "Privacy policy – GoatCounter",
	"terms":      "Terms of Service – GoatCounter",
	"contact":    "Contact – GoatCounter",
	"contribute": "Contribute – GoatCounter",
}

func (h website) tpl(w http.ResponseWriter, r *http.Request) error {
	t := r.URL.Path[1:]
	if t == "" {
		t = "home"
	}

	var loggedIn template.HTML
	if c, err := r.Cookie("key"); err == nil {
		var u goatcounter.User
		err = u.ByToken(r.Context(), c.Value)
		if err == nil {
			var s goatcounter.Site
			err = s.ByID(r.Context(), u.Site)
			if err == nil {
				loggedIn = template.HTML(fmt.Sprintf("Logged in as %s on <a href='%s'>%[2]s</a>",
					template.HTMLEscapeString(u.Name), template.HTMLEscapeString(s.URL())))
			}
		}
	}

	return zhttp.Template(w, t+".gohtml", struct {
		Globals
		Page     string
		MetaDesc string
		LoggedIn template.HTML
	}{newGlobals(w, r), t, metaDesc[t], loggedIn})
}

func (h website) status() func(w http.ResponseWriter, r *http.Request) error {
	started := goatcounter.Now()
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.JSON(w, map[string]string{
			"uptime":  goatcounter.Now().Sub(started).String(),
			"version": cfg.Version,
		})
	}
}

func (h website) signup(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "signup.gohtml", struct {
		Globals
		Page       string
		MetaDesc   string
		Site       goatcounter.Site
		User       goatcounter.User
		Validate   *zvalidate.Validator
		TuringTest string
	}{newGlobals(w, r), "signup", "Sign up for GoatCounter", goatcounter.Site{},
		goatcounter.User{}, nil, ""})
}

type signupArgs struct {
	Name       string `json:"site_name"`
	Code       string `json:"site_code"`
	Timezone   string `json:"timezone"`
	Email      string `json:"user_email"`
	UserName   string `json:"user_name"`
	TuringTest string `json:"turing_test"`
}

func (h website) doSignup(w http.ResponseWriter, r *http.Request) error {
	var args signupArgs
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.Site{Name: args.Name, Code: args.Code, Plan: cfg.Plan}
	user := goatcounter.User{Name: args.Name, Email: args.Email}

	v := zvalidate.New()
	if strings.TrimSpace(args.TuringTest) != "9" {
		v.Append("turing_test", "must fill in correct value")
		// Quick exit to prevent spurious errors/DB load from spambots.
		return zhttp.Template(w, "signup.gohtml", struct {
			Globals
			Page       string
			MetaDesc   string
			Site       goatcounter.Site
			User       goatcounter.User
			Validate   *zvalidate.Validator
			TuringTest string
		}{newGlobals(w, r), "signup", "Sign up for GoatCounter",
			site, user, &v, args.TuringTest})
	}

	txctx, tx, err := zdb.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create site.
	tz, err := tz.New(geo(r.RemoteAddr), args.Timezone)
	if err != nil {
		zlog.FieldsRequest(r).Fields(zlog.F{
			"timezone": args.Timezone,
			"args":     fmt.Sprintf("%#v\n", args),
		}).Error(err)
	}
	site.Settings = goatcounter.SiteSettings{Timezone: tz}

	err = site.Insert(txctx)
	if err != nil {
		if _, ok := err.(*zvalidate.Validator); !ok {
			return err
		}
		v.Sub("site", "", err)
	}

	// Create user.
	user.Site = site.ID
	err = user.Insert(txctx)
	if err != nil {
		if _, ok := err.(*zvalidate.Validator); !ok {
			return err
		}
		v.Sub("user", "", err)
		delete(v.Errors, "user.site")
	}

	if v.HasErrors() {
		return zhttp.Template(w, "signup.gohtml", struct {
			Globals
			Page       string
			MetaDesc   string
			Site       goatcounter.Site
			User       goatcounter.User
			Validate   *zvalidate.Validator
			TuringTest string
		}{newGlobals(w, r), "signup", "Sign up for GoatCounter", site, user, &v, args.TuringTest})
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	ctx := context.WithValue(r.Context(), ctxkey.Site, &site)
	err = user.RequestLogin(ctx)
	if err != nil {
		return err
	}
	go user.SendLoginMail(context.Background(), &site)

	return zhttp.SeeOther(w, fmt.Sprintf("%s/user/new?mailed=%s",
		site.URL(), url.QueryEscape(user.Email)))
}

func (h website) forgot(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "user_forgot.gohtml", struct {
		Globals
		Page     string
		MetaDesc string
	}{newGlobals(w, r), "forgot", "Forgot domain – GoatCounter"})
}

func (h website) doForgot(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Email string `json:"email"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	var users goatcounter.Users
	err = users.ByEmail(r.Context(), args.Email)
	if err != nil {
		return err
	}

	go func(ctx context.Context) {
		var body, name string
		if len(users) == 0 {
			body = fmt.Sprintf("There are no GoatCounter domains associated with ‘%s’", args.Email)
		} else {
			name = users[0].Name
			body = fmt.Sprintf("Sites associated with ‘%s’:\r\n\r\n", args.Email)
			for _, u := range users {
				var s goatcounter.Site
				err := s.ByID(ctx, u.Site)
				if err != nil {
					zlog.Error(err)
					continue
				}

				body += fmt.Sprintf("- %s\r\n\r\n", s.URL())
			}
		}

		err := zmail.Send("Your GoatCounter sites",
			mail.Address{Name: "GoatCounter login", Address: cfg.LoginFrom},
			[]mail.Address{{Name: name, Address: args.Email}},
			body)
		if err != nil {
			zlog.Errorf("zmail: %s", err)
		}
	}(goatcounter.NewContext(r.Context()))

	zhttp.Flash(w, "List of login URLs mailed to %s", args.Email)
	return zhttp.SeeOther(w, "/user/forgot")
}

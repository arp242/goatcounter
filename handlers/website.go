// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers // import "zgo.at/goatcounter/handlers"

import (
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstripe"
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

	r.Get("/contribute", zhttp.Wrap(h.contribute))
	r.Get("/status", zhttp.Wrap(h.status()))
	r.Get("/signup", zhttp.Wrap(h.signup))
	r.Post("/signup", zhttp.Wrap(h.doSignup))
	r.Get("/user/forgot", zhttp.Wrap(h.forgot))
	r.Post("/user/forgot", zhttp.Wrap(h.doForgot))
	r.Get("/code", zhttp.Wrap(h.code))
	for _, t := range []string{"", "help", "privacy", "terms", "contact", "gdpr"} {
		r.Get("/"+t, zhttp.Wrap(h.tpl))
	}
}

var metaDesc = map[string]string{
	"":           "Simple web statistics. No tracking of personal data.",
	"help":       "Help and support – GoatCounter",
	"privacy":    "Privacy policy – GoatCounter",
	"gdpr":       "GDPR consent notices – GoatCounter",
	"terms":      "Terms of Service – GoatCounter",
	"contact":    "Contact – GoatCounter",
	"contribute": "Contribute – GoatCounter",
	"code":       "Site integration code – GoatCounter",
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
					template.HTMLEscapeString(u.Email), template.HTMLEscapeString(s.URL())))
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
	Code       string `json:"site_code"`
	LinkDomain string `json:"link_domain"`
	Timezone   string `json:"timezone"`
	Email      string `json:"user_email"`
	Password   string `json:"password"`
	TuringTest string `json:"turing_test"`
}

func (h website) doSignup(w http.ResponseWriter, r *http.Request) error {
	var args signupArgs
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.Site{Code: args.Code, LinkDomain: args.LinkDomain, Plan: cfg.Plan}
	user := goatcounter.User{Email: args.Email, Password: []byte(args.Password)}

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
		var vErr *zvalidate.Validator
		if !errors.As(err, &vErr) {
			return err
		}
		v.Sub("site", "", err)
	}

	// Create user.
	user.Site = site.ID
	err = user.Insert(txctx)
	if err != nil {
		var vErr *zvalidate.Validator
		if !errors.As(err, &vErr) {
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

	ctx := goatcounter.WithSite(r.Context(), &site)
	err = user.RequestLogin(ctx)
	if err != nil {
		return err
	}

	err = user.Login(goatcounter.WithSite(r.Context(), &site))
	if err != nil {
		zlog.Errorf("login during account creation: %w", err)
	} else {
		zhttp.SetCookie(w, *user.LoginToken, site.Domain())
	}

	go func() {
		defer zlog.Recover()
		err := blackmail.Send("Welcome to GoatCounter!",
			blackmail.From("GoatCounter", cfg.EmailFrom),
			blackmail.To(user.Email),
			blackmail.BodyMustText(goatcounter.EmailTemplate("email_welcome.gotxt", struct {
				Site        goatcounter.Site
				User        goatcounter.User
				CountDomain string
			}{site, user, cfg.DomainCount})))
		if err != nil {
			zlog.Errorf("welcome email: %s", err)
		}
	}()

	return zhttp.SeeOther(w, fmt.Sprintf("%s/user/new", site.URL()))
}

func (h website) forgot(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "user_forgot_code.gohtml", struct {
		Globals
		Page     string
		MetaDesc string
	}{newGlobals(w, r), "forgot", "Forgot domain – GoatCounter"})
}

func (h website) code(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "code.gohtml", struct {
		Globals
		Page        string
		MetaDesc    string
		CountDomain string
		Site        goatcounter.Site
	}{newGlobals(w, r), "forgot", "Site integration code – GoatCounter",
		cfg.DomainCount, goatcounter.Site{Code: "MYCODE"}})
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

	var sites goatcounter.Sites
	for _, u := range users {
		var s goatcounter.Site
		err := s.ByID(r.Context(), u.Site)
		if zdb.ErrNoRows(err) { // Deleted site.
			continue
		}
		if err != nil {
			return err
		}

		sites = append(sites, s)
	}

	go func() {
		defer zlog.Recover()
		err := blackmail.Send("Your GoatCounter sites",
			mail.Address{Name: "GoatCounter", Address: cfg.EmailFrom},
			blackmail.To(args.Email),
			blackmail.BodyMustText(goatcounter.EmailTemplate("email_forgot_site.gotxt", struct {
				Sites goatcounter.Sites
				Email string
			}{sites, args.Email})))
		if err != nil {
			zlog.Error(err)
		}
	}()

	zhttp.Flash(w, "List of login URLs mailed to %s", args.Email)
	return zhttp.SeeOther(w, "/user/forgot")
}

func (h website) contribute(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "contribute.gohtml", struct {
		Globals
		Page            string
		MetaDesc        string
		StripePublicKey string
		SKU             string
	}{newGlobals(w, r), "contribute", "Contribute – GoatCounter",
		zstripe.PublicKey, stripePlans[cfg.Prod]["donate"]})
}

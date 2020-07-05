// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers // import "zgo.at/goatcounter/handlers"

import (
	"fmt"
	"html/template"
	"net/http"
	"net/mail"
	"os"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/zgoat/kommentaar/srvhttp"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/goatcounter/cfg"
	"zgo.at/guru"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zlog"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

type website struct{}

func (h website) Mount(r *chi.Mux, db zdb.DB) {
	r.Use(
		zhttp.RealIP,
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
	for _, t := range []string{"", "help", "privacy", "terms", "contact", "gdpr", "why", "data", "api"} {
		r.Get("/"+t, zhttp.Wrap(h.tpl))
	}

	r.With(zhttp.Ratelimit(zhttp.RatelimitOptions{
		Client:  zhttp.RatelimitIP,
		Store:   zhttp.NewRatelimitMemory(),
		Limit:   zhttp.RatelimitLimit(5, 86400),
		Message: "you can download this five times per day only",
	})).Get("/data/{file}", zhttp.Wrap(h.downloadData))

	conf := srvhttp.Args{
		Packages: []string{"./handlers"},
		Config:   "./kommentaar.conf",
		NoScan:   cfg.Prod,
		YAMLFile: "./docs/api.yaml",
		JSONFile: "./docs/api.json",
		HTMLFile: "./docs/api.html",
	}
	r.Get("/api.json", srvhttp.JSON(conf))
	r.Get("/api.yaml", srvhttp.YAML(conf))
	r.Get("/api.html", srvhttp.HTML(conf))
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
	"why":        "Why I made GoatCounter",
	"data":       "GoatCounter data",
	"api":        "API – GoatCounter",
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

	err = user.Login(goatcounter.WithSite(r.Context(), &site))
	if err != nil {
		zlog.Errorf("login during account creation: %w", err)
	} else {
		zhttp.SetCookie(w, *user.LoginToken, site.Domain())
	}

	bgrun.Run(func() {
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
	})

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

func (h website) downloadData(w http.ResponseWriter, r *http.Request) error {
	file := chi.URLParam(r, "file")
	if file == "" {
		return guru.New(400, "need file name")
	}
	switch file {
	case "ua.csv.gz", "bots.csv.gz", "screensize.csv.gz":
	default:
		return guru.Errorf(400, "unknown file name: %q", file)
	}

	fp, err := os.Open("/tmp/" + file)
	if err != nil {
		return err
	}
	defer fp.Close()

	err = header.SetContentDisposition(w.Header(), header.DispositionArgs{
		Type:     header.TypeAttachment,
		Filename: file,
	})
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/gzip")
	return zhttp.Stream(w, fp)
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

	bgrun.Run(func() {
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
	})

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

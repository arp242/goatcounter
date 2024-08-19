// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/mail"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"zgo.at/bgrun"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/guru"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/auth"
	"zgo.at/zhttp/mware"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zvalidate"
)

func NewWebsite(db zdb.DB, dev bool) chi.Router {
	r := chi.NewRouter()

	fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "", dev)
	if err != nil {
		panic(err)
	}

	website{fsys, true}.Mount(r, db, dev)
	return r
}

type website struct {
	templates fs.FS
	fromWWW   bool
}

func (h website) Mount(r chi.Router, db zdb.DB, dev bool) {
	r.Use(
		mware.RealIP(),
		mware.Unpanic(),
		middleware.RedirectSlashes,
		addctx(db, false, 10),
		mware.WrapWriter(),
		mware.Headers(nil))
	if dev {
		mware.RequestLog(nil)
	}

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(404, "Not Found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(405, "Method Not Allowed"))
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

	h.MountShared(r)

	r.Get("/signup", zhttp.Wrap(h.signup))
	r.Post("/signup", zhttp.Wrap(h.doSignup))
	r.Get("/user/forgot", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return h.forgot(nil, "", "")(w, r)
	}))
	r.Post("/user/forgot", zhttp.Wrap(h.doForgot))
	for _, t := range []string{"", "why", "design"} {
		r.Get("/"+t, zhttp.Wrap(h.tpl))
	}

	r.Get("/translating", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help/translating")
	}))
}

func (h website) MountShared(r chi.Router) {
	r.Get("/help", zhttp.Wrap(h.help))
	r.Get("/help/*", zhttp.Wrap(h.help))
	r.Get("/contribute", zhttp.Wrap(h.contribute))
	r.Get("/api.json", zhttp.Wrap(h.openAPI))
	r.Get("/api.html", zhttp.Wrap(h.openAPI))
	r.Get("/api2.html", zhttp.Wrap(h.openAPI))
	r.Post("/contact", zhttp.Wrap(h.contact))

	r.Get("/contact", zhttp.Wrap(h.tpl))

	r.Get("/terms", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help/terms")
	}))
	r.Get("/privacy", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help/privacy")
	}))
	r.Get("/gdpr", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help/gdpr")
	}))
	r.Get("/api", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help/api")
	}))
	r.Get("/code", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help")
	}))
	r.Get("/code/*", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/help/"+chi.URLParam(r, "*"))
	}))
}

var metaDesc = map[string]string{
	"":            "Simple web statistics. No tracking of personal data.",
	"privacy":     "Privacy policy – GoatCounter",
	"gdpr":        "GDPR consent notices – GoatCounter",
	"terms":       "Terms of Service – GoatCounter",
	"contact":     "Contact – GoatCounter",
	"contribute":  "Contribute – GoatCounter",
	"help":        "Documentation – GoatCounter",
	"why":         "Why I made GoatCounter",
	"api":         "API – GoatCounter",
	"design":      "GoatCounter design",
	"translating": "Translating GoatCounter",
}

func (h website) contact(w http.ResponseWriter, r *http.Request) error {
	type argsT struct {
		Email   string `json:"email"`
		Turing  string `json:"turing"`
		Message string `json:"message"`
		Return  string `json:"return"`
	}
	var args argsT

	render := func(reported error) error {
		var v zvalidate.Validator
		if !errors.As(reported, &v) {
			return reported
		}

		return zhttp.Template(w, "contact.gohtml", struct {
			Globals
			Page     string
			MetaDesc string
			Site     goatcounter.Site
			User     goatcounter.User

			Form     bool
			Args     argsT
			Validate *zvalidate.Validator
		}{newGlobals(w, r), "contact", "", goatcounter.Site{},
			goatcounter.User{},
			true, args,
			&v})
	}

	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	v := zvalidate.New()
	v.Required("email", args.Email)
	v.Required("turing", args.Turing)
	v.Required("message", args.Message)
	email := v.Email("email", args.Email)
	if strings.TrimSpace(args.Turing) != "9" {
		v.Append("turing", "must be 9")
	}
	v.Len("message", args.Message, 20, 0)
	if v.HasErrors() {
		return render(v)
	}

	err = blackmail.Send("GoatCounter message",
		email,
		blackmail.To("support@goatcounter.com"),
		blackmail.BodyText(append([]byte(args.Message), "\n"...)))
	if err != nil {
		return err
	}

	zhttp.Flash(w, "Message sent!")
	return zhttp.SeeOther(w, args.Return)
}

func (h website) openAPI(w http.ResponseWriter, r *http.Request) error {
	p := "tpl/" + path.Base(r.URL.Path)
	if p == "tpl/api.html" || p == "tpl/api2.html" {
		w.Header().Set("Content-Type", "text/html")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}

	fp, err := h.templates.Open(p)
	if err != nil {
		return guru.New(404, "Not Found")
	}
	defer fp.Close()

	d, err := io.ReadAll(fp)
	if err != nil {
		return err
	}

	if p == "tpl/api2.html" {
		url := "https://www.goatcounter.com"
		if s := goatcounter.GetSite(r.Context()); s != nil {
			url = s.URL(r.Context())
		}
		d = bytes.ReplaceAll(d, []byte(`spec-url=""`), []byte(fmt.Sprintf(`spec-url="%s/api.json"`, url)))
	}
	return zhttp.Bytes(w, d)
}

func (h website) tpl(w http.ResponseWriter, r *http.Request) error {
	t := path.Base(r.URL.Path[1:])
	if t == "" || t == "." {
		t = "home"
	}

	var loggedIn template.HTML
	if c, err := r.Cookie("key"); t == "home" && err == nil {
		var u goatcounter.User
		err = u.ByToken(r.Context(), c.Value)
		if err == nil {
			var s goatcounter.Site
			err = s.ByID(r.Context(), u.Site)
			if err == nil {
				loggedIn = template.HTML(fmt.Sprintf("Logged in as %s on <a href='%s'>%[2]s</a>",
					template.HTMLEscapeString(u.Email), template.HTMLEscapeString(s.URL(r.Context()))))
			}
		}
	}

	return zhttp.Template(w, t+".gohtml", struct {
		Globals
		Page     string
		MetaDesc string
		LoggedIn template.HTML

		// For the message form
		Validate *zvalidate.Validator
		Args     any
	}{newGlobals(w, r), t, metaDesc[t], loggedIn, nil, nil})
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

	site := goatcounter.Site{Code: args.Code, LinkDomain: args.LinkDomain}
	user := goatcounter.User{Email: args.Email, Password: []byte(args.Password),
		Access: goatcounter.UserAccesses{"all": goatcounter.AccessAdmin}}

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
	lookup := (goatcounter.Location{}).LookupIP(r.Context(), r.RemoteAddr)
	var cc string
	if len(lookup) > 0 {
		cc = strings.Split(lookup, "-")[0]
	}
	tz, err := tz.New(cc, args.Timezone)
	if err != nil {
		zlog.FieldsRequest(r).Fields(zlog.F{
			"timezone": args.Timezone,
			"args":     fmt.Sprintf("%#v\n", args),
		}).Error(err)
	}
	site.UserDefaults = goatcounter.UserSettings{
		Timezone:        tz,
		TwentyFourHours: true,
	}

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
	user.Settings = site.UserDefaults
	err = user.Insert(txctx, false)
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
		auth.SetCookie(w, *user.LoginToken, cookieDomain(&site, r))
	}

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.RunFunction("welcome email", func() {
		err := blackmail.Send("Welcome to GoatCounter!",
			blackmail.From("GoatCounter", goatcounter.Config(r.Context()).EmailFrom),
			blackmail.To(user.Email),
			blackmail.BodyMustText(goatcounter.TplEmailWelcome{ctx, site, user, goatcounter.Config(ctx).DomainCount}.Render),
		)
		if err != nil {
			zlog.Errorf("welcome email: %s", err)
		}
	})

	return zhttp.SeeOther(w, site.URL(r.Context())+"/user/new")
}

func (h website) forgot(err error, email, turingTest string) zhttp.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		v := zvalidate.As(err)
		if v != nil {
			err = nil
		}
		return zhttp.Template(w, "user_forgot_code.gohtml", struct {
			Globals
			Page       string
			MetaDesc   string
			Err        error
			Validate   *zvalidate.Validator
			Email      string
			TuringTest string
		}{newGlobals(w, r), "forgot", "Forgot domain – GoatCounter",
			err, v, email, turingTest})
	}
}

func (h website) doForgot(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Email      string `json:"email"`
		TuringTest string `json:"turing_test"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return h.forgot(err, args.Email, args.TuringTest)(w, r)
	}

	v := zvalidate.New()
	v.Required("email", args.Email)
	v.Required("turing_test", args.TuringTest)
	v.Email("email", args.Email)
	if strings.TrimSpace(args.TuringTest) != "9" {
		v.Append("turing", "must be 9")
	}
	if v.HasErrors() {
		return h.forgot(v, args.Email, args.TuringTest)(w, r)
	}

	var users goatcounter.Users
	err = users.ByEmail(r.Context(), args.Email)
	if err != nil {
		return h.forgot(err, args.Email, args.TuringTest)(w, r)
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

	ctx := goatcounter.CopyContextValues(r.Context())
	bgrun.RunFunction("email:sites", func() {
		defer zlog.Recover()
		err := blackmail.Send("Your GoatCounter sites",
			mail.Address{Name: "GoatCounter", Address: goatcounter.Config(ctx).EmailFrom},
			blackmail.To(args.Email),
			blackmail.BodyMustText(goatcounter.TplEmailForgotSite{ctx, sites, args.Email}.Render))
		if err != nil {
			zlog.Error(err)
		}
	})

	zhttp.Flash(w, "List of login URLs mailed to %s", args.Email)
	return zhttp.SeeOther(w, "/user/forgot")
}

func (h website) help(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.GetSite(r.Context())
	if site == nil {
		site = &goatcounter.Site{Code: "MYCODE"}
	}

	dc := goatcounter.Config(r.Context()).DomainCount
	if dc == "" {
		dc = Site(r.Context()).SchemelessURL(r.Context())
	}

	cp := chi.URLParam(r, "*")
	if cp == "" {
		return zhttp.MovedPermanently(w, "/help/start")
	}

	{
		fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "tpl", goatcounter.Config(r.Context()).Dev)
		if err != nil {
			return err
		}
		fsys, err = zfs.SubIfExists(fsys, "tpl/help")
		if err != nil {
			return err
		}

		_, err = fs.Stat(fsys, cp+".md")
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			w.WriteHeader(404)
			cp = "404"
		}
	}

	return zhttp.Template(w, "help.gohtml", struct {
		Globals
		Page        string
		CodePage    string
		MetaDesc    string
		CountDomain string
		SiteURL     string
		SiteDomain  string
		FromWWW     bool

		// For the message form
		Validate *zvalidate.Validator
		Args     any
	}{newGlobals(w, r), "help", cp, "Documentation – GoatCounter",
		dc, site.URL(r.Context()), site.Domain(r.Context()), h.fromWWW,
		nil, nil})
}

func (h website) contribute(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "contribute.gohtml", struct {
		Globals
		Page     string
		MetaDesc string
		FromWWW  bool
	}{newGlobals(w, r), "contribute", "Contribute – GoatCounter", h.fromWWW})
}

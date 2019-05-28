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
	r.Get("/user/new", zhttp.Wrap(h.new))
	r.Post("/user/requestlogin", zhttp.Wrap(h.requestLogin))
	r.Get("/user/login/{key}", zhttp.Wrap(h.login))
	a := r.With(filterLoggedIn)
	a.Post("/user/logout", zhttp.Wrap(h.logout))

	//r.Post("/user/create", zhttp.Wrap(h.create))
	// a = r.With(zhttp.Filter(func(r *http.Request) bool {
	// 	u := goatcounter.GetUser(r.Context())
	// 	return u != nil && u.ID > 0 && u.Role == goatcounter.UserRoleAdmin
	// }))
	// a.Get("/admin", zhttp.Wrap(h.admin))
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

	var url = fmt.Sprintf("%s/user/login/%s", cfg.Domain, *u.LoginKey)
	go func() {
		err := smail.Send("Your login URL",
			mail.Address{Name: "", Address: "TODO@example.com"},
			[]mail.Address{{Name: u.Name, Address: u.Email}},
			fmt.Sprintf("Hi there,\n\nYour login URL for Goatcounter is:\n\n  %s\n\nGo to it to log in.\n", url))
		if err != nil {
			zlog.Errorf("smail: %s", err)
		}
	}()

	if cfg.Prod {
		zhttp.Flash(w,
			"All good. Login URL emailed to %q; please click it in the next 15 minutes to continue.",
			u.Email)
	} else {
		zhttp.Flash(w, url)
	}
	return zhttp.SeeOther(w, "/")
}

func (h user) create(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		SiteName   string `json:"site_name"`
		SiteDomain string `json:"site_domain"`
		SiteCode   string `json:"site_domain"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	var s goatcounter.Site
	s.Code = args.SiteCode
	s.Name = args.SiteName
	s.Domain = args.SiteDomain
	err = s.Insert(r.Context())
	if err != nil {
		return err
	}

	var u goatcounter.User
	u.Email = args.Email
	u.Name = args.Name
	err = u.Insert(r.Context())
	if err != nil {
		return err
	}

	err = u.RequestLogin(r.Context())
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://%s.%s/user/login/%s", s.Code, cfg.Domain, *u.LoginKey)
	go func() {
		err := smail.Send("Your login URL",
			mail.Address{Name: "", Address: "TODO@example.com"},
			[]mail.Address{{Name: u.Name, Address: u.Email}},
			fmt.Sprintf("Hi there,\n\nYour login URL for Goatcounter is:\n\n  %s\n\nGo to it to log in.\n", url))
		if err != nil {
			zlog.Errorf("smail: %s", err)
		}
	}()

	if cfg.Prod {
		zhttp.Flash(w,
			"All good. Login URL emailed to %q; please click it in the next 15 minutes to continue.",
			u.Email)
	} else {
		zhttp.Flash(w, url)
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

func (h user) admin(w http.ResponseWriter, r *http.Request) error {
	var sites goatcounter.Sites
	err := sites.List(r.Context())
	if err != nil {
		return err
	}

	var users goatcounter.Users
	err = users.ListAllSites(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "admin.gohtml", struct {
		Globals
		Sites goatcounter.Sites
		Users goatcounter.Users
	}{newGlobals(w, r), sites, users})
}

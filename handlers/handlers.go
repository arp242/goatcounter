package handlers

import (
	"html/template"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zhttp"
)

type Globals struct {
	User    *goatcounter.User
	Site    *goatcounter.Site
	Flash   template.HTML
	Static  string
	Domain  string
	Version string
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	g := Globals{
		User:    goatcounter.GetUser(r.Context()),
		Site:    goatcounter.GetSite(r.Context()),
		Flash:   zhttp.ReadFlash(w, r),
		Static:  cfg.DomainStatic,
		Domain:  cfg.Domain,
		Version: cfg.Version,
	}
	// TODO: not sure why this is needed?
	if g.User == nil {
		g.User = &goatcounter.User{}
	}
	return g
}

func NewSite(db *sqlx.DB) chi.Router {
	if !cfg.Prod {
		packTpl = nil
	}
	zhttp.InitTpl(packTpl)

	r := chi.NewRouter()
	Website{}.Mount(r, db)
	return r
}

func NewStatic(dir, domain string, prod bool) chi.Router {
	r := chi.NewRouter()
	if !prod {
		packPublic = nil
	}
	r.Get("/*", zhttp.NewStatic(dir, domain, packPublic).ServeHTTP)
	return r
}

func NewBackend(db *sqlx.DB) chi.Router {
	r := chi.NewRouter()
	Backend{}.Mount(r, db)
	return r
}

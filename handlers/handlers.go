package handlers

import (
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
	Path    string
	Flash   *zhttp.FlashMessage
	Static  string
	Domain  string
	Version string
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	g := Globals{
		User:    goatcounter.GetUser(r.Context()),
		Site:    goatcounter.GetSite(r.Context()),
		Path:    r.URL.Path,
		Flash:   zhttp.ReadFlash(w, r),
		Static:  cfg.DomainStatic,
		Domain:  cfg.Domain,
		Version: cfg.Version,
	}
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
	cache := 0
	if cfg.Prod {
		cache = 86400 * 30
	}
	// Use * for Access-Control-Allow-Origin as we can't use *.domain, which is
	// needed to allow "code.domain", "code2.domain", etc.
	r.Get("/*", zhttp.NewStatic(dir, "*", cache, packPublic).ServeHTTP)
	return r
}

func NewBackend(db *sqlx.DB) chi.Router {
	r := chi.NewRouter()
	Backend{}.Mount(r, db)
	return r
}

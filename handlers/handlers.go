// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/pack"
	"zgo.at/zhttp"
	"zgo.at/zstripe"
)

type Globals struct {
	Context context.Context
	User    *goatcounter.User
	Site    *goatcounter.Site
	Path    string
	Flash   *zhttp.FlashMessage
	Static  string
	Domain  string
	Version string
	Billing bool
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	g := Globals{
		Context: r.Context(),
		User:    goatcounter.GetUser(r.Context()),
		Site:    goatcounter.GetSite(r.Context()),
		Path:    r.URL.Path,
		Flash:   zhttp.ReadFlash(w, r),
		Static:  strings.Split(cfg.DomainStatic, ",")[0],
		Domain:  cfg.Domain,
		Version: cfg.Version,
		Billing: zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "",
	}
	if g.User == nil {
		g.User = &goatcounter.User{}
	}
	return g
}

func NewWebsite(db *sqlx.DB) chi.Router {
	if !cfg.Prod {
		pack.Templates = nil
	}
	zhttp.InitTpl(pack.Templates)

	r := chi.NewRouter()
	website{}.Mount(r, db)
	return r
}

func NewStatic(dir, domain string, prod bool) chi.Router {
	r := chi.NewRouter()
	if !prod {
		pack.Public = nil
	}
	cache := 0
	if cfg.Prod {
		cache = 86400 * 30
	}
	// Use * for Access-Control-Allow-Origin as we can't use *.domain, which is
	// needed to allow "code.domain", "code2.domain", etc.
	r.Get("/*", zhttp.NewStatic(dir, "*", cache, pack.Public).ServeHTTP)
	return r
}

func NewBackend(db *sqlx.DB) chi.Router {
	r := chi.NewRouter()
	backend{}.Mount(r, db)
	return r
}

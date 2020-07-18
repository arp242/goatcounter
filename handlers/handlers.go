// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/pack"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstripe"
)

type Globals struct {
	Context        context.Context
	User           *goatcounter.User
	Site           *goatcounter.Site
	HasUpdates     bool
	Path           string
	Flash          *zhttp.FlashMessage
	Static         string
	StaticDomain   string
	Domain         string
	Version        string
	Billing        bool
	GoatcounterCom bool
	Dev            bool
	Port           string
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	g := Globals{
		Context:        r.Context(),
		User:           goatcounter.GetUser(r.Context()),
		Site:           goatcounter.GetSite(r.Context()),
		Path:           r.URL.Path,
		Flash:          zhttp.ReadFlash(w, r),
		Static:         cfg.URLStatic,
		Domain:         cfg.Domain,
		Version:        cfg.Version,
		Billing:        zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "",
		GoatcounterCom: cfg.GoatcounterCom,
		Dev:            !cfg.Prod,
		Port:           cfg.Port,
	}
	if g.User == nil {
		g.User = &goatcounter.User{}
	}
	if cfg.DomainStatic == "" {
		g.StaticDomain = goatcounter.GetSite(r.Context()).Domain()
	} else {
		g.StaticDomain = cfg.DomainStatic
	}

	var err error
	g.HasUpdates, err = (new(goatcounter.Updates)).HasSince(r.Context(), g.User.SeenUpdatesAt)
	if err != nil {
		zlog.FieldsRequest(r).Error(err)
	}

	return g
}

func NewWebsite(db zdb.DB) chi.Router {
	r := chi.NewRouter()
	website{}.Mount(r, db)
	return r
}

func NewStatic(r chi.Router, dir string, prod bool) chi.Router {
	var cache map[string]int
	if prod {
		cache = map[string]int{
			"/count.js": 86400,
			"*":         86400 * 30,
		}
	}
	r.Get("/*", zhttp.NewStatic(dir, "*", cache, pack.Public).ServeHTTP)
	return r
}

func NewBackend(db zdb.DB, acmeh http.HandlerFunc) chi.Router {
	r := chi.NewRouter()
	backend{}.Mount(r, db)

	if acmeh != nil {
		r.Get("/.well-known/acme-challenge/{key}", acmeh)
	}

	if !cfg.GoatcounterCom {
		NewStatic(r, "./public", cfg.Prod)
	}

	return r
}

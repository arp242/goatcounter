// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

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
	Context    context.Context
	User       *goatcounter.User
	Site       *goatcounter.Site
	HasUpdates bool
	Path       string
	Flash      *zhttp.FlashMessage
	Static     string
	Domain     string
	Version    string
	Billing    bool
	Saas       bool
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	g := Globals{
		Context: r.Context(),
		User:    goatcounter.GetUser(r.Context()),
		Site:    goatcounter.GetSite(r.Context()),
		Path:    r.URL.Path,
		Flash:   zhttp.ReadFlash(w, r),
		Static:  cfg.DomainStatic[0],
		Domain:  cfg.Domain,
		Version: cfg.Version,
		Billing: zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "",
		Saas:    cfg.Saas,
	}
	if g.User == nil {
		g.User = &goatcounter.User{}
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

func NewStatic(dir, domain string, prod bool) chi.Router {
	r := chi.NewRouter()
	cache := 0
	if prod {
		cache = 86400 * 30
	}
	// Use * for Access-Control-Allow-Origin as we can't use *.domain, which is
	// needed to allow "code.domain", "code2.domain", etc.
	r.Get("/*", zhttp.NewStatic(dir, "*", cache, pack.Public).ServeHTTP)
	return r
}

func NewBackend(db zdb.DB) chi.Router {
	r := chi.NewRouter()
	backend{}.Mount(r, db)
	return r
}

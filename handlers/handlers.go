// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"zgo.at/goatcounter"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zstripe"
)

// Site calls goatcounter.MustGetSite; it's just shorter :-)
func Site(ctx context.Context) *goatcounter.Site    { return goatcounter.MustGetSite(ctx) }
func Account(ctx context.Context) *goatcounter.Site { return goatcounter.GetAccount(ctx) }
func User(ctx context.Context) *goatcounter.User    { return goatcounter.MustGetUser(ctx) }

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
		Static:         goatcounter.Config(r.Context()).URLStatic,
		Domain:         goatcounter.Config(r.Context()).Domain,
		Version:        goatcounter.Version,
		Billing:        zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "",
		GoatcounterCom: goatcounter.Config(r.Context()).GoatcounterCom,
		Dev:            goatcounter.Config(r.Context()).Dev,
		Port:           goatcounter.Config(r.Context()).Port,
	}
	if g.User == nil {
		g.User = &goatcounter.User{}
	}
	if goatcounter.Config(r.Context()).DomainStatic == "" {
		g.StaticDomain = goatcounter.GetSite(r.Context()).Domain(r.Context())
	} else {
		g.StaticDomain = goatcounter.Config(r.Context()).DomainStatic
	}

	var err error
	g.HasUpdates, err = (new(goatcounter.Updates)).HasSince(r.Context(), g.User.SeenUpdatesAt)
	if err != nil {
		zlog.FieldsRequest(r).Error(err)
	}

	return g
}

func NewStatic(r chi.Router, dev bool) chi.Router {
	var cache map[string]int
	if !dev {
		cache = map[string]int{
			"/count.js": 86400,
			"*":         86400 * 30,
		}
	}
	fsys, err := zfs.EmbedOrDir(goatcounter.Static, "public", dev)
	if err != nil {
		panic(err)
	}

	r.Get("/*", zhttp.NewStatic("*", fsys, cache).ServeHTTP)
	return r
}

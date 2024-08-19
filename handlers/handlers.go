// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"zgo.at/goatcounter/v2"
	"zgo.at/z18n"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zstd/zfs"
)

var rateLimits = struct {
	count, api, apiCount, export, login func(*http.Request) (int, int64)
}{
	count:    mware.RatelimitLimit(4, 1),
	api:      mware.RatelimitLimit(4, 1),
	apiCount: mware.RatelimitLimit(60, 120),
	export:   mware.RatelimitLimit(1, 3600),
	login:    mware.RatelimitLimit(20, 60),
}

// Set the rate limits.
func SetRateLimit(name string, reqs int, secs int64) {
	r := mware.RatelimitLimit(reqs, secs)
	switch strings.ToLower(name) {
	case "count":
		rateLimits.count = r
	case "api":
		rateLimits.api = r
	case "apicount", "api-count":
		rateLimits.apiCount = r
	case "export":
		rateLimits.export = r
	case "login":
		rateLimits.login = r
	default:
		panic(fmt.Sprintf("handlers.SetRateLimit: invalid name: %q", name))
	}
}

// Site calls goatcounter.MustGetSite; it's just shorter :-)
func Site(ctx context.Context) *goatcounter.Site    { return goatcounter.MustGetSite(ctx) }
func Account(ctx context.Context) *goatcounter.Site { return goatcounter.MustGetAccount(ctx) }
func User(ctx context.Context) *goatcounter.User    { return goatcounter.MustGetUser(ctx) }

var T = z18n.T

type Globals struct {
	Context        context.Context
	User           *goatcounter.User
	Site           *goatcounter.Site
	Path           string
	Base           string
	Flash          *zhttp.FlashMessage
	Static         string
	StaticDomain   string
	Domain         string
	Version        string
	GoatcounterCom bool
	Dev            bool
	Port           string
	Websocket      bool
	JSTranslations map[string]string
	HideUI         bool
}

func (g Globals) T(msg string, data ...any) template.HTML {
	return template.HTML(z18n.T(g.Context, msg, data...))
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	ctx := r.Context()
	base := goatcounter.Config(ctx).BasePath
	path := strings.TrimPrefix(r.URL.Path, base)
	if path == "" {
		path = "/"
	}
	g := Globals{
		Context:        ctx,
		User:           goatcounter.GetUser(ctx),
		Site:           goatcounter.GetSite(ctx),
		Path:           path,
		Base:           base,
		Flash:          zhttp.ReadFlash(w, r),
		Static:         goatcounter.Config(ctx).URLStatic,
		Domain:         goatcounter.Config(ctx).Domain,
		Version:        goatcounter.Version,
		GoatcounterCom: goatcounter.Config(ctx).GoatcounterCom,
		Dev:            goatcounter.Config(ctx).Dev,
		Port:           goatcounter.Config(ctx).Port,
		Websocket:      goatcounter.Config(ctx).Websocket,
		HideUI:         r.URL.Query().Get("hideui") != "",
		JSTranslations: map[string]string{
			"error/date-future":           T(ctx, "error/date-future|That would be in the future"),
			"error/date-past":             T(ctx, "error/date-past|That would be before the site’s creation; GoatCounter is not *that* good ;-)"),
			"error/date-mismatch":         T(ctx, "error/date-mismatch|end date is before start date"),
			"error/load-url":              T(ctx, "error/load-url|Could not load %(url): %(error)", z18n.P{"url": "%(url)", "error": "%(error)"}),
			"notify/saved":                T(ctx, "notify/saved|Saved!"),
			"dashboard/future":            T(ctx, "dashboard/future|future"),
			"dashboard/tooltip-event":     T(ctx, "dashboard/tooltip-event|%(unique) clicks; %(clicks) total clicks", z18n.P{"unique": "%(unique)", "clicks": "%(clicks)"}),
			"dashboard/totals/num-visits": T(ctx, "dashboard/totals/num-visits|%(num-visits) visits", z18n.P{"num-visits": "%(num-visits)"}),
			"datepicker/keyboard":         T(ctx, "datepicker/keyboard|Use the arrow keys to pick a date"),
			"datepicker/month-prev":       T(ctx, "datepicker/month-prev|Previous month"),
			"datepicker/month-next":       T(ctx, "datepicker/month-next|Next month"),
		},
	}
	if g.User == nil {
		g.User = &goatcounter.User{}
	}
	if goatcounter.Config(r.Context()).DomainStatic == "" {
		s := goatcounter.GetSite(r.Context())
		if s != nil {
			g.StaticDomain = s.Domain(r.Context())
		} else {
			g.StaticDomain = "/"
		}
	} else {
		g.StaticDomain = goatcounter.Config(r.Context()).DomainStatic
	}

	return g
}

func NewStatic(r chi.Router, dev, goatcounterCom bool, basePath string) chi.Router {
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

	s := zhttp.NewStatic("*", fsys, cache)
	s.Header("/count.js", map[string]string{
		"Cross-Origin-Resource-Policy": "cross-origin",
	})
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, basePath)
		s.ServeHTTP(w, r)
	})
	return r
}

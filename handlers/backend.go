// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/mssola/user_agent"
	"github.com/teamwork/guru"
	"github.com/teamwork/utils/httputilx/header"
	"github.com/teamwork/validate"
	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
)

type Backend struct{}

func (h Backend) Mount(r chi.Router, db *sqlx.DB) {
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		addctx(db, true),
		middleware.RedirectSlashes)

	{
		rr := r.With(zhttp.Headers(nil))
		rr.Get("/robots.txt", zhttp.HandlerRobots([][]string{{"User-agent: *", "Disallow: /"}}))
		rr.Post("/csp", zhttp.HandlerCSP())
		rr.Get("/count", zhttp.Wrap(h.count))
	}

	{
		headers := http.Header{
			"Strict-Transport-Security": []string{"max-age=2592000"},
			"X-Frame-Options":           []string{"deny"},
			"X-Content-Type-Options":    []string{"nosniff"},
		}
		header.SetCSP(headers, header.CSPArgs{
			header.CSPDefaultSrc: {header.CSPSourceNone},
			header.CSPImgSrc:     {cfg.DomainStatic, "https://static.goatcounter.com"},
			header.CSPScriptSrc:  {cfg.DomainStatic},
			header.CSPStyleSrc:   {cfg.DomainStatic, header.CSPSourceUnsafeInline}, // style="height: " on the charts.
			header.CSPFontSrc:    {cfg.DomainStatic},
			header.CSPFormAction: {header.CSPSourceSelf},
			header.CSPConnectSrc: {header.CSPSourceSelf},
			header.CSPReportURI:  {"/csp"},
		})

		a := r.With(zhttp.Headers(headers), zhttp.Log(true, ""), keyAuth)
		user{}.mount(a)
		{
			ap := a.With(loggedInOrPublic)
			ap.Get("/", zhttp.Wrap(h.index))
			ap.Get("/refs", zhttp.Wrap(h.refs))
			ap.Get("/pages", zhttp.Wrap(h.pages))
			ap.Get("/browsers", zhttp.Wrap(h.browsers))
		}
		{
			af := a.With(loggedIn)
			af.Get("/settings", zhttp.Wrap(h.settings))
			af.Post("/save", zhttp.Wrap(h.save))
			af.Get("/export/{file}", zhttp.Wrap(h.export))
			af.Post("/add", zhttp.Wrap(h.add))
			af.Get("/remove/{id}", zhttp.Wrap(h.removeConfirm))
			af.Post("/remove/{id}", zhttp.Wrap(h.remove))
			af.With(admin).Get("/admin", zhttp.Wrap(h.admin))
		}
	}

	{
		aa := r.With(zhttp.Log(true, ""), keyAuth, admin)
		//aa.Get("/debug/pprof/*", pprof.Index)
		aa.Get("/debug/*", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/debug/pprof") {
				pprof.Index(w, r)
			}
			zhttp.SeeOther(w, fmt.Sprintf("/debug/pprof/%s?%s",
				r.URL.Path[7:], r.URL.Query().Encode()))
		})
		aa.Get("/debug/pprof/cmdline", pprof.Cmdline)
		aa.Get("/debug/pprof/profile", pprof.Profile)
		aa.Get("/debug/pprof/symbol", pprof.Symbol)
		aa.Get("/debug/pprof/trace", pprof.Trace)
	}
}

// Use GIF because it's the smallest filesize (PNG is 116 bytes, vs 43 for GIF).
var gif = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x1, 0x0, 0x1, 0x0, 0x80,
	0x1, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff, 0xff, 0x21, 0xf9, 0x4, 0x1, 0xa, 0x0,
	0x1, 0x0, 0x2c, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x1, 0x0, 0x0, 0x2, 0x2, 0x4c,
	0x1, 0x0, 0x3b}

func (h Backend) count(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store,no-cache")

	// Don't track pages fetched with the browser's prefetch algorithm.
	// See https://github.com/usefathom/fathom/issues/13
	if r.Header.Get("X-Moz") == "prefetch" || r.Header.Get("X-Purpose") == "preview" || user_agent.New(r.UserAgent()).Bot() {
		w.Header().Set("Content-Type", "image/gif")
		return zhttp.Bytes(w, gif)
	}

	var hit goatcounter.Hit
	_, err := zhttp.Decode(r, &hit)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		return err
	}
	hit.Site = goatcounter.MustGetSite(r.Context()).ID
	hit.CreatedAt = time.Now().UTC()

	goatcounter.Memstore.Append(hit, goatcounter.Browser{
		Site:      hit.Site,
		Browser:   r.UserAgent(),
		CreatedAt: hit.CreatedAt,
	})

	w.Header().Set("Content-Type", "image/gif")
	return zhttp.Bytes(w, gif)
}

const day = 24 * time.Hour

func (h Backend) index(w http.ResponseWriter, r *http.Request) error {
	// Cache much more aggressively for public displays. Don't care so much if
	// it's outdated by an hour.
	if goatcounter.MustGetSite(r.Context()).Settings.Public &&
		goatcounter.GetUser(r.Context()).ID == 0 {
		w.Header().Set("Cache-Control", "public,max-age=3600")
		w.Header().Set("Vary", "Cookie")
	}

	var (
		start = time.Now().Add(-7 * day)
		end   = time.Now()
	)
	// Use period first as fallback when there's no JS.
	if p := r.URL.Query().Get("period"); p != "" {
		switch p {
		case "day":
			// Do nothing.
		case "week":
			start = start.Add(-7 * day)
		case "month":
			start = start.Add(-30 * day)
		case "quarter":
			start = start.Add(-91 * day)
		case "half-year":
			start = start.Add(-183 * day)
		case "year":
			start = start.Add(-365 * day)
		case "all":
			start = start.Add(-365 * day * 20) // TODO: set to 1970
		}
	} else {
		if s := r.URL.Query().Get("period-start"); s != "" {
			var err error
			start, err = time.Parse("2006-01-02", s)
			if err != nil {
				zhttp.FlashError(w, "start date: %s", err.Error())
				start = time.Now().Add(-7 * day)
			}
		}
		if s := r.URL.Query().Get("period-end"); s != "" {
			var err error
			end, err = time.Parse("2006-01-02", s)
			if err != nil {
				zhttp.FlashError(w, "end date: %s", err.Error())
				end = time.Now()
			}
		}
	}

	l := zlog.Module("backend")

	var pages goatcounter.HitStats
	total, totalDisplay, _, err := pages.List(r.Context(), start, end, nil)
	if err != nil {
		return err
	}

	var browsers goatcounter.BrowserStats
	totalBrowsers, err := browsers.List(r.Context(), start, end)
	if err != nil {
		return err
	}

	// Add refers.
	sr := r.URL.Query().Get("showrefs")
	var refs goatcounter.HitStats
	var moreRefs bool
	if sr != "" {
		moreRefs, err = refs.ListRefs(r.Context(), sr, start, end, 0)
		if err != nil {
			return err
		}
	}

	subs, err := goatcounter.MustGetSite(r.Context()).ListSubs(r.Context())
	if err != nil {
		return err
	}

	l = l.Since("fetch data")
	x := zhttp.Template(w, "backend.gohtml", struct {
		Globals
		ShowRefs         string
		Period           string
		PeriodStart      time.Time
		PeriodEnd        time.Time
		Pages            goatcounter.HitStats
		Refs             goatcounter.HitStats
		MoreRefs         bool
		TotalHits        int
		TotalHitsDisplay int
		Browsers         goatcounter.BrowserStats
		TotalBrowsers    uint64
		SubSites         []string
	}{newGlobals(w, r), sr, r.URL.Query().Get("hl-period"), start, end, pages,
		refs, moreRefs, total, totalDisplay, browsers, totalBrowsers, subs})
	l = l.Since("exec template")
	return x
}

func (h Backend) admin(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var a goatcounter.AdminStats
	err := a.List(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_admin.gohtml", struct {
		Globals
		Stats goatcounter.AdminStats
	}{newGlobals(w, r), a})
}

func (h Backend) refs(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		o2, err := strconv.ParseInt(o, 10, 32)
		if err != nil {
			return err
		}
		offset = int(o2)
	}

	var refs goatcounter.HitStats
	more, err := refs.ListRefs(r.Context(), r.URL.Query().Get("showrefs"), start, end, offset)
	if err != nil {
		return err
	}

	tpl, err := zhttp.ExecuteTpl("_backend_refs.gohtml", refs)
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]interface{}{
		"rows": string(tpl),
		"more": more,
	})
}

func (h Backend) browsers(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	var browsers goatcounter.BrowserStats
	total, err := browsers.ListBrowser(r.Context(), r.URL.Query().Get("browser"), start, end)
	if err != nil {
		return err
	}

	tpl := zhttp.FuncMap["vbar_chart"].(func(goatcounter.BrowserStats, uint64) template.HTML)(browsers, total)

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h Backend) pages(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	var pages goatcounter.HitStats
	_, totalDisplay, more, err := pages.List(r.Context(), start, end,
		strings.Split(r.URL.Query().Get("exclude"), ","))
	if err != nil {
		return err
	}

	tpl, err := zhttp.ExecuteTpl("_backend_pages.gohtml", struct {
		Pages       goatcounter.HitStats
		PeriodStart time.Time
		PeriodEnd   time.Time

		// Dummy values so template won't error out.
		Refs     bool
		ShowRefs string
	}{pages, start, end, false, ""})
	if err != nil {
		return err
	}

	paths := make([]string, len(pages))
	for i := range pages {
		paths[i] = pages[i].Path
	}

	return zhttp.JSON(w, map[string]interface{}{
		"rows":          string(tpl),
		"paths":         paths,
		"total_display": totalDisplay,
		"more":          more,
	})
}

func (h Backend) settings(w http.ResponseWriter, r *http.Request) error {
	var sites goatcounter.Sites
	err := sites.ListSubs(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_settings.gohtml", struct {
		Globals
		SubSites goatcounter.Sites
	}{newGlobals(w, r), sites})
}

func (h Backend) save(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		Name     string                   `json:"name"`
		Settings goatcounter.SiteSettings `json:"settings"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.MustGetSite(r.Context())
	site.Name = args.Name
	site.Settings = args.Settings

	err = site.Update(r.Context())
	if err != nil {
		zhttp.FlashError(w, "%v", err)
	} else {
		zhttp.Flash(w, "Saved!")
	}

	return zhttp.SeeOther(w, "/settings")
}

func (h Backend) export(w http.ResponseWriter, r *http.Request) error {
	file := strings.ToLower(chi.URLParam(r, "file"))

	w.Header().Set("Content-Type", "text/csv")
	err := header.SetContentDisposition(w.Header(), header.DispositionArgs{
		Type:     header.TypeAttachment,
		Filename: file,
	})
	if err != nil {
		return err
	}

	c := csv.NewWriter(w)
	switch file {
	default:
		return guru.Errorf(400, "unknown export file: %#v", file)

	case "hits.csv":
		var hits goatcounter.Hits
		err := hits.List(r.Context())
		if err != nil {
			return err
		}
		c.Write([]string{"Path", "Referrer (sanitized)", "Referrer query params", "Original Referrer", "Date (RFC 3339/ISO 8601)"})
		for _, hit := range hits {
			rp := ""
			if hit.RefParams != nil {
				rp = *hit.RefParams
			}
			ro := ""
			if hit.RefOriginal != nil {
				ro = *hit.RefOriginal
			}
			c.Write([]string{hit.Path, hit.Ref, rp, ro, hit.CreatedAt.Format(time.RFC3339)})
		}

	case "browsers.csv":
		var browsers goatcounter.Browsers
		err := browsers.List(r.Context())
		if err != nil {
			return err
		}

		c.Write([]string{"User-Agent string", "Date (RFC 3339/ISO 8601)"})
		for _, browser := range browsers {
			c.Write([]string{browser.Browser, browser.CreatedAt.Format(time.RFC3339)})
		}
	}

	c.Flush()
	return c.Error()
}

func (h Backend) removeConfirm(w http.ResponseWriter, r *http.Request) error {
	v := validate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var s goatcounter.Site
	err := s.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_remove.gohtml", struct {
		Globals
		Site goatcounter.Site
	}{newGlobals(w, r), s})
}

func (h Backend) remove(w http.ResponseWriter, r *http.Request) error {
	v := validate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var s goatcounter.Site
	err := s.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	err = s.Delete(r.Context())
	if err != nil {
		return err
	}

	zhttp.Flash(w, "Site removed")
	return zhttp.SeeOther(w, "/settings")
}

func (h Backend) add(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.Site{
		Code:   args.Code,
		Name:   args.Name,
		Parent: &goatcounter.MustGetSite(r.Context()).ID,
		Plan:   goatcounter.PlanChild,
	}
	err = site.Insert(r.Context())
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, "/settings")
	}

	zhttp.Flash(w, "Site added")
	return zhttp.SeeOther(w, "/settings")
}

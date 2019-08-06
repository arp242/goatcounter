// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"encoding/csv"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/mssola/user_agent"
	"github.com/teamwork/guru"
	"github.com/teamwork/utils/httputilx/header"
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
		middleware.RedirectSlashes,
		addctx(db, true))

	rr := r.With(zhttp.Headers(nil))

	// Don't allow any indexing of the backend interface by search engines.
	rr.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "public,max-age=31536000")
		w.WriteHeader(200)
		w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})

	// CSP errors.
	rr.Post("/csp", func(w http.ResponseWriter, r *http.Request) {
		d, _ := ioutil.ReadAll(r.Body)
		zlog.Errorf("CSP error: %s", string(d))
		w.WriteHeader(202)
	})

	// Counter that the script on the website calls.
	rr.Get("/count", zhttp.Wrap(h.count))

	// Backend interface.
	a := r.With(
		zhttp.Headers(http.Header{
			"Strict-Transport-Security": []string{"max-age=2592000"},
			"X-Frame-Options":           []string{"deny"},
			"X-Content-Type-Options":    []string{"nosniff"},
			"Content-Security-Policy": {header.CSP{
				header.CSPDefaultSrc: {header.CSPSourceNone},
				header.CSPImgSrc:     {cfg.DomainStatic},
				header.CSPScriptSrc:  {cfg.DomainStatic},
				header.CSPStyleSrc:   {cfg.DomainStatic, header.CSPSourceUnsafeInline}, // style="height: " on the charts.
				header.CSPFontSrc:    {cfg.DomainStatic},
				header.CSPFormAction: {header.CSPSourceSelf},
				header.CSPConnectSrc: {header.CSPSourceSelf},
				header.CSPReportURI:  {"/csp"},
			}.String()},
		}),
		zhttp.Log(true, ""),
		keyAuth)

	a.Get("/", zhttp.Wrap(h.index))
	a.Get("/refs", zhttp.Wrap(h.refs))
	a.Get("/pages", zhttp.Wrap(h.pages))
	a.Get("/settings", zhttp.Wrap(h.settings))
	a.Post("/save", zhttp.Wrap(h.save))
	a.Get("/export/{file}", zhttp.Wrap(h.export))

	user{}.mount(a)
}

// Gif is the smallest filesize (PNG is 116 bytes, vs 43).
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
	// TODO: need more processing to be truly useful, so disable for now.
	// Collect it anyway for my own purpose: sniff out any bots or other "weird"
	// stuff that we don't want to count.
	// err = browsers.List(r.Context(), start, end)
	// if err != nil {
	// 	return err
	// }

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
	}{newGlobals(w, r), sr, r.URL.Query().Get("hl-period"), start, end, pages,
		refs, moreRefs, total, totalDisplay, browsers})
	l = l.Since("exec template")
	return x
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
	return zhttp.Template(w, "backend_settings.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

func (h Backend) save(w http.ResponseWriter, r *http.Request) error {
	args := struct {
		Domain   string                   `json:"domain"`
		Settings goatcounter.SiteSettings `json:"settings"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	site := goatcounter.MustGetSite(r.Context())
	site.Domain = args.Domain
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

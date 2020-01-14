// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/arp242/geoip2-golang"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/monoculum/formam"
	"github.com/mssola/user_agent"
	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/httputilx/header"
	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

type backend struct{}

func (h backend) Mount(r chi.Router, db *sqlx.DB) {
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		addctx(db, true),
		middleware.RedirectSlashes)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, 404, errors.New("Not Found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, 405, errors.New("Method Not Allowed"))
	})
	r.Get("/status", zhttp.Wrap(h.status()))

	{
		rr := r.With(zhttp.Headers(nil))
		rr.Get("/robots.txt", zhttp.HandlerRobots([][]string{{"User-agent: *", "Disallow: /"}}))
		rr.Post("/jserr", zhttp.HandlerJSErr())
		rr.Post("/csp", zhttp.HandlerCSP())

		// 4 pageviews/second should be more than enough.
		rr.With(zhttp.Ratelimit(zhttp.RatelimitOptions{
			Client: func(r *http.Request) string {
				// Add in the User-Agent to reduce the problem of multiple
				// people in the same building hitting the limit.
				return r.RemoteAddr + r.UserAgent()
			},
			Store: zhttp.NewRatelimitMemory(),
			Limit: func(r *http.Request) (int, int64) {
				if r.RemoteAddr == "127.0.0.1" { // From zbuf
					return 1 << 14, 1
				}
				return 4, 1
			},
		})).Get("/count", zhttp.Wrap(h.count))

		if cfg.CertDir != "" {
			zhttp.MountACME(rr, cfg.CertDir)
		}
	}

	{
		headers := http.Header{
			"Strict-Transport-Security": []string{"max-age=2592000"},
			"X-Frame-Options":           []string{"deny"},
			"X-Content-Type-Options":    []string{"nosniff"},
		}
		st := strings.Split(cfg.DomainStatic, ",")
		// https://stripe.com/docs/security#content-security-policy
		header.SetCSP(headers, header.CSPArgs{
			header.CSPDefaultSrc: {header.CSPSourceNone},
			header.CSPImgSrc:     st,
			header.CSPScriptSrc:  append(st, "data:", "https://chat.goatcounter.com", "https://js.stripe.com"),
			header.CSPStyleSrc:   append(st, header.CSPSourceUnsafeInline), // style="height: " on the charts.
			header.CSPFontSrc:    st,
			header.CSPFormAction: {header.CSPSourceSelf},
			header.CSPConnectSrc: {header.CSPSourceSelf, "https://chat.goatcounter.com", "https://api.stripe.com"},
			header.CSPFrameSrc:   {"https://js.stripe.com", "https://hooks.stripe.com"},
			// Too much noise: header.CSPReportURI:  {"/csp"},
		})

		a := r.With(zhttp.Headers(headers), keyAuth)
		if !cfg.Prod {
			a = a.With(zhttp.Log(true, ""))
		}

		user{}.mount(a)
		{
			ap := a.With(loggedInOrPublic)
			ap.Get("/", zhttp.Wrap(h.index))
			ap.Get("/refs", zhttp.Wrap(h.refs))
			ap.Get("/pages", zhttp.Wrap(h.pages))
			ap.Get("/browsers", zhttp.Wrap(h.browsers))
			ap.Get("/sizes", zhttp.Wrap(h.sizes))
			ap.Get("/locations", zhttp.Wrap(h.locations))
		}
		{
			af := a.With(loggedIn)
			if zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "" {
				billing{}.mount(a, af)
			}
			af.Get("/updates", zhttp.Wrap(h.updates))
			af.Get("/settings", zhttp.Wrap(h.settings))
			af.Post("/save-settings", zhttp.Wrap(h.saveSettings))
			af.With(zhttp.Ratelimit(zhttp.RatelimitOptions{
				Client:  zhttp.RatelimitIP,
				Store:   zhttp.NewRatelimitMemory(),
				Limit:   zhttp.RatelimitLimit(1, 3600*4),
				Message: "you can request only one export every 4 hours",
			})).Get("/export/{file}", zhttp.Wrap(h.export))
			af.Post("/add", zhttp.Wrap(h.addSubsite))
			af.Get("/remove/{id}", zhttp.Wrap(h.removeSubsiteConfirm))
			af.Post("/remove/{id}", zhttp.Wrap(h.removeSubsite))
			af.Get("/purge", zhttp.Wrap(h.purgeConfirm))
			af.Post("/purge", zhttp.Wrap(h.purge))
			af.With(admin).Get("/admin", zhttp.Wrap(h.admin))
			af.With(admin).Get("/admin/{id}", zhttp.Wrap(h.adminSite))
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

var geodb = func() *geoip2.Reader {
	g, err := geoip2.FromBytes(pack.GeoDB)
	if err != nil {
		panic(err)
	}
	return g
}()

func geo(ip string) string {
	loc, err := geodb.Country(net.ParseIP(ip))
	if err != nil && cfg.Prod {
		zlog.Module("geo").Field("ip", ip).Error(err)
	}
	return loc.Country.IsoCode
}

func (h backend) status() func(w http.ResponseWriter, r *http.Request) error {
	started := time.Now().UTC()
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.JSON(w, map[string]string{
			"uptime":  time.Now().UTC().Sub(started).String(),
			"version": cfg.Version,
		})
	}
}

func (h backend) count(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store,no-cache")

	// Don't track pages fetched with the browser's prefetch algorithm.
	// See https://github.com/usefathom/fathom/issues/13
	if r.Header.Get("X-Moz") == "prefetch" || r.Header.Get("X-Purpose") == "preview" {
		w.Header().Set("Content-Type", "image/gif")
		return zhttp.Bytes(w, gif)
	}

	// I think this is some sort of bot, but log/verify to be sure.
	if r.UserAgent() == "" || r.UserAgent() == "''" || len(r.UserAgent()) < 5 {
		zlog.
			Field("URL", r.URL.String()).
			Field("headers", fmt.Sprintf("%#v", r.Header)).
			Print("empty User-Agent")
	}

	hit := goatcounter.Hit{
		Site:      goatcounter.MustGetSite(r.Context()).ID,
		Browser:   r.UserAgent(),
		Location:  geo(r.RemoteAddr),
		CountRef:  r.Referer(),
		CreatedAt: time.Now().UTC(),
	}
	if user_agent.New(r.UserAgent()).Bot() {
		hit.Bot = 1
	}

	_, err := zhttp.Decode(r, &hit)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		return err
	}
	goatcounter.Memstore.Append(hit)

	w.Header().Set("Content-Type", "image/gif")
	return zhttp.Bytes(w, gif)
}

const day = 24 * time.Hour

func (h backend) index(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	// Cache much more aggressively for public displays. Don't care so much if
	// it's outdated by an hour.
	if site.Settings.Public && goatcounter.GetUser(r.Context()).ID == 0 {
		w.Header().Set("Cache-Control", "public,max-age=3600")
		w.Header().Set("Vary", "Cookie")
	}

	var (
		start = time.Now().UTC().Add(-7 * day)
		end   = time.Now().UTC()
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
			start = time.Date(1970, 1, 1, 0, 0, 0, 0, start.Location())
		}
	} else {
		if s := r.URL.Query().Get("period-start"); s != "" {
			var err error
			start, err = time.Parse("2006-01-02", s)
			if err != nil {
				zhttp.FlashError(w, "start date: %s", err.Error())
				start = time.Now().UTC().Add(-7 * day)
			}
		}
		if s := r.URL.Query().Get("period-end"); s != "" {
			var err error
			end, err = time.Parse("2006-01-02", s)
			if err != nil {
				zhttp.FlashError(w, "end date: %s", err.Error())
				end = time.Now().UTC()
			}
		}
	}

	filter := r.URL.Query().Get("filter")

	l := zlog.Module("backend").Field("site", site.ID)

	var pages goatcounter.HitStats
	total, totalDisplay, _, err := pages.List(r.Context(), start, end, filter, nil)
	if err != nil {
		return err
	}
	l = l.Since("pages.List")

	var browsers goatcounter.Stats
	totalBrowsers, totalMobile, err := browsers.ListBrowsers(r.Context(), start, end)
	if err != nil {
		return err
	}
	l = l.Since("browsers.List")

	var sizeStat goatcounter.Stats
	totalSize, err := sizeStat.ListSizes(r.Context(), start, end)
	if err != nil {
		return err
	}
	l = l.Since("sizeStat.ListSizes")

	var locStat goatcounter.Stats
	totalLoc, err := locStat.ListLocations(r.Context(), start, end)
	if err != nil {
		return err
	}
	showMoreLoc := len(locStat) > 0 && float32(locStat[len(locStat)-1].Count)/float32(totalLoc)*100 < 3.0
	l = l.Since("locStat.List")

	// Add refers.
	sr := r.URL.Query().Get("showrefs")
	var refs goatcounter.HitStats
	var moreRefs bool
	if sr != "" {
		moreRefs, err = refs.ListRefs(r.Context(), sr, start, end, 0)
		if err != nil {
			return err
		}
		l = l.Since("refs.ListRefs")
	}

	subs, err := site.ListSubs(r.Context())
	if err != nil {
		return err
	}

	x := zhttp.Template(w, "backend.gohtml", struct {
		Globals
		ShowRefs          string
		Period            string
		PeriodStart       time.Time
		PeriodEnd         time.Time
		Filter            string
		Pages             goatcounter.HitStats
		Refs              goatcounter.HitStats
		MoreRefs          bool
		TotalHits         int
		TotalHitsDisplay  int
		Browsers          goatcounter.Stats
		TotalBrowsers     int
		TotalMobile       string
		SubSites          []string
		SizeStat          goatcounter.Stats
		TotalSize         int
		LocationStat      goatcounter.Stats
		TotalLocation     int
		ShowMoreLocations bool
	}{newGlobals(w, r), sr, r.URL.Query().Get("hl-period"), start, end, filter,
		pages, refs, moreRefs, total, totalDisplay, browsers, totalBrowsers,
		fmt.Sprintf("%.1f", float32(totalMobile)/float32(totalBrowsers)*100),
		subs, sizeStat, totalSize, locStat, totalLoc, showMoreLoc})
	l = l.Since("zhttp.Template")
	l.FieldsSince().Print("")
	return x
}

func (h backend) admin(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	if !cfg.PgSQL { // TODO
		return guru.New(400, "not implemented in SQLite yet")
	}

	var a goatcounter.AdminStats
	err := a.List(r.Context(), r.URL.Query().Get("order"))
	if err != nil {
		return err
	}

	var cs goatcounter.AdminCountRefs
	err = cs.List(r.Context())
	if err != nil {
		return err
	}

	contacts := make([]string, len(a))
	for i := range a {
		contacts[i] = a[i].Email
	}
	sliceutil.UniqString(contacts)

	return zhttp.Template(w, "backend_admin.gohtml", struct {
		Globals
		Stats     goatcounter.AdminStats
		CountRefs goatcounter.AdminCountRefs
		Contacts  string
	}{newGlobals(w, r), a, cs, strings.Join(contacts, ", ")})
}

func (h backend) adminSite(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	if !cfg.PgSQL { // TODO
		return guru.New(400, "not implemented in SQLite yet")
	}

	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var a goatcounter.AdminSiteStat
	err := a.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_admin_site.gohtml", struct {
		Globals
		Stat goatcounter.AdminSiteStat
	}{newGlobals(w, r), a})
}

func (h backend) refs(w http.ResponseWriter, r *http.Request) error {
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

func (h backend) browsers(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	var browsers goatcounter.Stats
	total, err := browsers.ListBrowser(r.Context(), r.URL.Query().Get("name"), start, end)
	if err != nil {
		return err
	}

	f := zhttp.FuncMap["hbar_chart"].(func(goatcounter.Stats, int, int, float32, bool) template.HTML)
	t, _ := strconv.ParseInt(r.URL.Query().Get("total"), 10, 64)
	tpl := f(browsers, total, int(t), .5, true)

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) sizes(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	var sizeStat goatcounter.Stats
	total, err := sizeStat.ListSize(r.Context(), r.URL.Query().Get("name"), start, end)
	if err != nil {
		return err
	}

	f := zhttp.FuncMap["hbar_chart"].(func(goatcounter.Stats, int, int, float32, bool) template.HTML)
	t, _ := strconv.ParseInt(r.URL.Query().Get("total"), 10, 64)
	tpl := f(sizeStat, total, int(t), .5, true)

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) locations(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	var locStat goatcounter.Stats
	total, err := locStat.ListLocations(r.Context(), start, end)
	if err != nil {
		return err
	}

	f := zhttp.FuncMap["hbar_chart"].(func(goatcounter.Stats, int, int, float32, bool) template.HTML)
	tpl := f(locStat, total, total, 0, false)
	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) pages(w http.ResponseWriter, r *http.Request) error {
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
		r.URL.Query().Get("filter"), strings.Split(r.URL.Query().Get("exclude"), ","))
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

func (h backend) updates(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())

	var up goatcounter.Updates
	err := up.List(r.Context(), u.SeenUpdatesAt)
	if err != nil {
		return err
	}

	err = u.SeenUpdates(r.Context())
	if err != nil {
		zlog.Field("user", fmt.Sprintf("%d", u.ID)).Error(err)
	}

	return zhttp.Template(w, "backend_updates.gohtml", struct {
		Globals
		Updates goatcounter.Updates
	}{newGlobals(w, r), up})
}

func (h backend) settings(w http.ResponseWriter, r *http.Request) error {
	return h.settingsTpl(w, r, nil)
}

func (h backend) settingsTpl(w http.ResponseWriter, r *http.Request, verr *zvalidate.Validator) error {
	var sites goatcounter.Sites
	err := sites.ListSubs(r.Context())
	if err != nil {
		return err
	}

	var merr map[string][]string
	if verr != nil {
		merr = verr.Errors
	}

	return zhttp.Template(w, "backend_settings.gohtml", struct {
		Globals
		SubSites goatcounter.Sites
		Validate map[string][]string
	}{newGlobals(w, r), sites, merr})
}

func (h backend) saveSettings(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()

	args := struct {
		Name     string                   `json:"name"`
		Cname    string                   `json:"cname"`
		Settings goatcounter.SiteSettings `json:"settings"`
		User     goatcounter.User         `json:"user"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		ferr, ok := err.(*formam.Error)
		if !ok || ferr.Code() != formam.ErrCodeConversion {
			return err
		}
		v.Append(ferr.Path(), "must be a number")

		// TODO: we return here because formam stops decoding on the first
		// error. We should really fix this in formam, but it's an incompatible
		// change.
		return h.settingsTpl(w, r, &v)
	}

	txctx, tx, err := zdb.Begin(r.Context())
	defer tx.Rollback()

	user := goatcounter.GetUser(txctx)
	user.Name = args.User.Name
	user.Email = args.User.Email
	err = user.Update(txctx)
	if err != nil {
		if _, ok := err.(*zvalidate.Validator); !ok {
			return err
		}
		v.Sub("user", "", err)
	}

	site := goatcounter.MustGetSite(txctx)
	site.Name = args.Name
	site.Settings = args.Settings
	if args.Cname != "" && !site.PlanCustomDomain(txctx) {
		return guru.New(http.StatusForbidden, "need a business plan to set custom domain")
	}

	if args.Cname == "" {
		site.Cname = nil
	} else {
		if site.Cname == nil || *site.Cname != args.Cname {
			acme.Domains <- args.Cname
		}
		site.Cname = &args.Cname
	}

	err = site.Update(txctx)
	if err != nil {
		if _, ok := err.(*zvalidate.Validator); !ok {
			return err
		}
		v.Sub("site", "", err)
	}

	if v.HasErrors() {
		return h.settingsTpl(w, r, &v)
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	zhttp.Flash(w, "Saved!")
	return zhttp.SeeOther(w, "/settings")
}

func (h backend) export(w http.ResponseWriter, r *http.Request) error {
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
		c.Write([]string{"Path", "Referrer (sanitized)",
			"Referrer query params", "Original Referrer", "Browser",
			"Screen size", "Date (RFC 3339/ISO 8601)"})
		for _, hit := range hits {
			rp := ""
			if hit.RefParams != nil {
				rp = *hit.RefParams
			}
			ro := ""
			if hit.RefOriginal != nil {
				ro = *hit.RefOriginal
			}
			c.Write([]string{hit.Path, hit.Ref, rp, ro, hit.Browser,
				sliceutil.JoinFloat(hit.Size), hit.CreatedAt.Format(time.RFC3339)})
		}
	}

	c.Flush()
	return c.Error()
}

func (h backend) removeSubsiteConfirm(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
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

func (h backend) removeSubsite(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
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

	zhttp.Flash(w, "Site ‘%s ’removed.", s.Name)
	return zhttp.SeeOther(w, "/settings#tab-additional-sites")
}

func (h backend) addSubsite(w http.ResponseWriter, r *http.Request) error {
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
		return zhttp.SeeOther(w, "/settings#tab-additional-sites")
	}

	zhttp.Flash(w, "Site ‘%s’ added.", site.Name)
	return zhttp.SeeOther(w, "/settings#tab-additional-sites")
}

func (h backend) purgeConfirm(w http.ResponseWriter, r *http.Request) error {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	var list goatcounter.HitStats
	err := list.ListPathsLike(r.Context(), path)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_purge.gohtml", struct {
		Globals
		PurgePath string
		List      goatcounter.HitStats
	}{newGlobals(w, r), path, list})
}

func (h backend) purge(w http.ResponseWriter, r *http.Request) error {
	var list goatcounter.Hits
	err := list.Purge(r.Context(), r.Form.Get("path"))
	if err != nil {
		return err
	}

	zhttp.Flash(w, "Done!")
	return zhttp.SeeOther(w, "/settings#tab-purge")
}

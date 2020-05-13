// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arp242/geoip2-golang"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/monoculum/formam"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/errors"
	"zgo.at/goatcounter/pack"
	"zgo.at/guru"
	"zgo.at/isbot"
	"zgo.at/tz"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/syncutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

type backend struct{}

// Always use the daily view if the number of days is larger than this.
const DailyView = 90

func (h backend) Mount(r chi.Router, db zdb.DB) {
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		addctx(db, true),
		middleware.RedirectSlashes,
		zhttp.NoStore,
		zhttp.WrapWriter)

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
		rateLimited := rr.With(zhttp.Ratelimit(zhttp.RatelimitOptions{
			Client: func(r *http.Request) string {
				// Add in the User-Agent to reduce the problem of multiple
				// people in the same building hitting the limit.
				return r.RemoteAddr + r.UserAgent()
			},
			Store: zhttp.NewRatelimitMemory(),
			Limit: func(r *http.Request) (int, int64) {
				if !cfg.Prod {
					return 1 << 32, 1
				}
				// From httpbuf
				// TODO: in some setups this may always be true, e.g. when proxy
				// through nginx without settings this properly. Need to check.
				if r.RemoteAddr == "127.0.0.1" {
					return 1 << 14, 1
				}
				return 4, 1
			},
		}))
		countHandler := zhttp.Wrap(h.count)
		rateLimited.Get("/count", countHandler)
		rateLimited.Post("/count", countHandler) // to support navigator.sendBeacon (JS)
	}

	{
		headers := http.Header{
			"Strict-Transport-Security": []string{"max-age=2592000"},
			"X-Frame-Options":           []string{"deny"},
			"X-Content-Type-Options":    []string{"nosniff"},
		}
		// https://stripe.com/docs/security#content-security-policy
		ds := []string{""}
		if cfg.DomainStatic == "" {
			ds[0] = header.CSPSourceSelf
		} else {
			ds[0] = cfg.DomainStatic
		}
		gc := "https://gc.goatcounter.com"
		if !cfg.Prod {
			gc = "http://gc." + cfg.Domain
		}
		header.SetCSP(headers, header.CSPArgs{
			header.CSPDefaultSrc: {header.CSPSourceNone},
			header.CSPImgSrc:     append(ds, "data:", gc),
			header.CSPScriptSrc: append(ds, "https://chat.goatcounter.com", "https://js.stripe.com",
				// Inline GoatCounter setup
				"https://gc.zgo.at", "'sha256-rhp1kopsm+UqtrN5qCeSn81YXeO4wJtXDvQE00OrLoQ='"),
			header.CSPStyleSrc:    append(ds, header.CSPSourceUnsafeInline), // style="height: " on the charts.
			header.CSPFontSrc:     ds,
			header.CSPFormAction:  {header.CSPSourceSelf},
			header.CSPConnectSrc:  {header.CSPSourceSelf, "https://chat.goatcounter.com", "https://api.stripe.com"},
			header.CSPFrameSrc:    {"https://js.stripe.com", "https://hooks.stripe.com"},
			header.CSPManifestSrc: ds,
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
			ap.Get("/toprefs", zhttp.Wrap(h.topRefs))
			ap.Get("/pages-by-ref", zhttp.Wrap(h.pagesByRef))
		}
		{
			af := a.With(loggedIn)
			if zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "" {
				billing{}.mount(a, af)
			}
			af.Get("/updates", zhttp.Wrap(h.updates))
			af.Get("/settings", zhttp.Wrap(h.settings))
			af.Get("/code", zhttp.Wrap(h.code))
			af.Get("/ip", zhttp.Wrap(h.ip))
			af.Post("/save-settings", zhttp.Wrap(h.saveSettings))
			af.With(zhttp.Ratelimit(zhttp.RatelimitOptions{
				Client:  zhttp.RatelimitIP,
				Store:   zhttp.NewRatelimitMemory(),
				Limit:   zhttp.RatelimitLimit(1, 3600*24),
				Message: "you can request only one export a day",
			})).Post("/start-export", zhttp.Wrap(h.startExport))
			af.Get("/download-export", zhttp.Wrap(h.downloadExport))
			af.Post("/add", zhttp.Wrap(h.addSubsite))
			af.Get("/remove/{id}", zhttp.Wrap(h.removeSubsiteConfirm))
			af.Post("/remove/{id}", zhttp.Wrap(h.removeSubsite))
			af.Get("/purge", zhttp.Wrap(h.purgeConfirm))
			af.Post("/purge", zhttp.Wrap(h.purge))
			af.Post("/delete", zhttp.Wrap(h.delete))
			af.With(admin).Get("/admin", zhttp.Wrap(h.admin))
			af.With(admin).Get("/admin/sql", zhttp.Wrap(h.adminSQL))
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
	loc, _ := geodb.Country(net.ParseIP(ip))
	return loc.Country.IsoCode
}

func (h backend) status() func(w http.ResponseWriter, r *http.Request) error {
	started := goatcounter.Now()
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.JSON(w, map[string]string{
			"uptime":  goatcounter.Now().Sub(started).String(),
			"version": cfg.Version,
		})
	}
}

func (h backend) count(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "image/gif")

	bot := isbot.Bot(r)

	// Don't track pages fetched with the browser's prefetch algorithm.
	if bot == isbot.BotPrefetch {
		return zhttp.Bytes(w, gif)
	}

	site := goatcounter.MustGetSite(r.Context())
	for _, ip := range site.Settings.IgnoreIPs {
		if ip == r.RemoteAddr {
			w.Header().Add("X-Goatcounter", fmt.Sprintf("ignored because %q is in the IP ignore list", ip))
			w.WriteHeader(http.StatusAccepted)
			return zhttp.Bytes(w, gif)
		}
	}

	hit := goatcounter.Hit{
		Site:      site.ID,
		Browser:   r.UserAgent(),
		Location:  geo(r.RemoteAddr),
		CreatedAt: goatcounter.Now(),
	}

	err := formam.NewDecoder(&formam.DecoderOptions{TagName: "json"}).Decode(r.URL.Query(), &hit)
	if err != nil {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("error decoding parameters: %s", err))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}
	if hit.Bot > 0 && hit.Bot < 150 {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("wrong value: b=%d", hit.Bot))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}

	if isbot.Is(bot) { // Prefer the backend detection.
		hit.Bot = int(bot)
	}

	if uint8(hit.Bot) >= isbot.BotJSNightmare {
		zlog.Module("jsbot").Fields(zlog.F{
			"bot": hit.Bot,
			"ip":  r.RemoteAddr,
			"ua":  r.UserAgent(),
			"h":   zlog.JSON(jsonutil.MustMarshal(r.Header)),
			"url": r.RequestURI,
		}).Printf("")
	}

	// TODO: move to memstore?
	{
		var sess goatcounter.Session
		started, err := sess.GetOrCreate(r.Context(), hit.Path, r.UserAgent(), zhttp.RemovePort(r.RemoteAddr))
		if err != nil {
			zlog.Error(err)
		}

		hit.Session = &sess.ID
		if started {
			hit.FirstVisit = zdb.Bool(true)
		}
		if !started {
			first, err := sess.HasPath(r.Context(), hit.Path)
			if err != nil {
				zlog.Error(err)
			}
			if first {
				hit.FirstVisit = zdb.Bool(true)
			}
		}
	}

	err = hit.Validate(r.Context())
	if err != nil {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("not valid: %s", err))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}

	goatcounter.Memstore.Append(hit)
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

	start, end, err := getPeriod(w, r, site)
	if err != nil {
		zhttp.FlashError(w, err.Error())
	}
	if start.IsZero() || end.IsZero() {
		y, m, d := goatcounter.Now().In(site.Settings.Timezone.Loc()).Date()
		now := time.Date(y, m, d, 0, 0, 0, 0, site.Settings.Timezone.Loc())
		start = now.Add(-7 * day).UTC()
		end = time.Date(y, m, d, 23, 59, 59, 9, now.Location()).UTC()
	}

	filter := r.URL.Query().Get("filter")
	daily, forcedDaily := getDaily(r, start, end)

	startl := zlog.Module("dashboard")
	l := zlog.Module("dashboard").Field("site", site.ID)

	var (
		wg                              sync.WaitGroup
		pages                           goatcounter.HitStats
		total, totalDisplay             int
		totalUnique, totalUniqueDisplay int
		morePages                       bool
		pagesErr                        error
	)
	wg.Add(1)
	go func() {
		defer zlog.Recover()
		defer wg.Done()

		total, totalUnique, totalDisplay, totalUniqueDisplay, morePages, pagesErr = pages.List(r.Context(), start, end, filter, nil)
		//l = l.Since("pages.List")
	}()

	var browsers goatcounter.Stats
	totalBrowsers, err := browsers.ListBrowsers(r.Context(), start, end)
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

	var topRefs goatcounter.Stats
	totalTopRefs, showMoreRefs, err := topRefs.ListRefs(r.Context(), start, end, 10, 0)
	if err != nil {
		return err
	}
	l = l.Since("topRefs.List")

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

	cd := cfg.DomainCount
	if cd == "" {
		cd = goatcounter.MustGetSite(r.Context()).Domain()
		if cfg.Port != "" {
			cd += ":" + cfg.Port
		}
	}

	syncutil.Wait(r.Context(), &wg)
	if pagesErr != nil {
		return pagesErr
	}

	l = startl.Since("get data")

	// TODO: this is getting a bit silly ... should split this out by rendering
	// partials.
	x := zhttp.Template(w, "backend.gohtml", struct {
		Globals
		CountDomain        string
		ShowRefs           string
		SelectedPeriod     string
		PeriodStart        time.Time
		PeriodEnd          time.Time
		Filter             string
		Pages              goatcounter.HitStats
		MorePages          bool
		Refs               goatcounter.HitStats
		MoreRefs           bool
		TotalHits          int
		TotalUniqueHits    int
		TotalHitsDisplay   int
		TotalUniqueDisplay int
		Browsers           goatcounter.Stats
		TotalBrowsers      int
		SubSites           []string
		SizeStat           goatcounter.Stats
		TotalSize          int
		LocationStat       goatcounter.Stats
		TotalLocation      int
		ShowMoreLocations  bool
		TopRefs            goatcounter.Stats
		TotalTopRefs       int
		ShowMoreRefs       bool
		Daily              bool
		ForcedDaily        bool
	}{newGlobals(w, r), cd, sr, r.URL.Query().Get("hl-period"), start, end,
		filter, pages, morePages, refs, moreRefs, total, totalUnique,
		totalDisplay, totalUniqueDisplay, browsers, totalBrowsers, subs,
		sizeStat, totalSize, locStat, totalLoc, showMoreLoc, topRefs,
		totalTopRefs, showMoreRefs, daily, forcedDaily})
	l.Since("zhttp.Template")
	return x
}

func (h backend) topRefs(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, goatcounter.MustGetSite(r.Context()))
	if err != nil {
		return err
	}

	var refs goatcounter.Stats
	o, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	total, hasMore, err := refs.ListRefs(r.Context(), start, end, 10, int(o))
	if err != nil {
		return err
	}

	t, _ := strconv.ParseInt(r.URL.Query().Get("total"), 10, 64)
	tpl := goatcounter.HorizontalChart(r.Context(), refs, total, int(t), 0, true, false)

	return zhttp.JSON(w, map[string]interface{}{
		"html":     string(tpl),
		"has_more": hasMore,
	})
}

func (h backend) pagesByRef(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, goatcounter.MustGetSite(r.Context()))
	if err != nil {
		return err
	}

	var hits goatcounter.Stats
	total, err := hits.ByRef(r.Context(), start, end, r.URL.Query().Get("name"))
	if err != nil {
		return err
	}

	tpl := goatcounter.HorizontalChart(r.Context(), hits, total, total, 1, true, true)

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) admin(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	l := zlog.Module("admin")

	var a goatcounter.AdminStats
	err := a.List(r.Context(), r.URL.Query().Get("order"))
	if err != nil {
		return err
	}
	l = l.Since("stats")

	var sites goatcounter.Sites
	err = sites.List(r.Context())
	if err != nil {
		return err
	}
	grouped := make(map[string]int) // day → count
	for _, s := range sites {
		if s.Parent != nil {
			continue
		}
		grouped[s.CreatedAt.Format("2006-01-02")]++
	}

	var (
		signups    []goatcounter.Stat
		maxSignups int
	)
	for k, v := range grouped {
		if v > maxSignups {
			maxSignups = v
		}
		signups = append(signups, goatcounter.Stat{
			Day:          k,
			Hourly:       []int{v},
			HourlyUnique: []int{v},
		})
	}
	sort.Slice(signups, func(i, j int) bool {
		return signups[i].Day < signups[j].Day
	})

	l = l.Since("signups")

	l.FieldsSince().Debug("admin")
	return zhttp.Template(w, "backend_admin.gohtml", struct {
		Globals
		Stats      goatcounter.AdminStats
		Signups    []goatcounter.Stat
		MaxSignups int
	}{newGlobals(w, r), a, signups, maxSignups})
}

func (h backend) adminSQL(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var a goatcounter.AdminPgStats
	err := a.List(r.Context(), r.URL.Query().Get("order"))
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_admin_sql.gohtml", struct {
		Globals
		Stats goatcounter.AdminPgStats
	}{newGlobals(w, r), a})
}

func (h backend) adminSite(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var code string
	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		code = chi.URLParam(r, "id")
	}

	var a goatcounter.AdminSiteStat
	var err error
	if id > 0 {
		err = a.ByID(r.Context(), id)
	} else {
		err = a.ByCode(r.Context(), code)
	}
	if err != nil {
		if zdb.ErrNoRows(err) {
			return guru.New(404, "no such site")
		}
		return err
	}

	return zhttp.Template(w, "backend_admin_site.gohtml", struct {
		Globals
		Stat goatcounter.AdminSiteStat
	}{newGlobals(w, r), a})
}

func (h backend) refs(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, goatcounter.MustGetSite(r.Context()))
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

	tpl, err := zhttp.ExecuteTpl("_backend_refs.gohtml", map[string]interface{}{
		"Refs": refs,
		"Site": goatcounter.MustGetSite(r.Context()),
	})
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]interface{}{
		"rows": string(tpl),
		"more": more,
	})
}

func (h backend) browsers(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, goatcounter.MustGetSite(r.Context()))
	if err != nil {
		return err
	}

	var browsers goatcounter.Stats
	total, err := browsers.ListBrowser(r.Context(), r.URL.Query().Get("name"), start, end)
	if err != nil {
		return err
	}

	t, _ := strconv.ParseInt(r.URL.Query().Get("total"), 10, 64)
	tpl := goatcounter.HorizontalChart(r.Context(), browsers, total, int(t), .2, true, false)

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) sizes(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, goatcounter.MustGetSite(r.Context()))
	if err != nil {
		return err
	}

	var sizeStat goatcounter.Stats
	total, err := sizeStat.ListSize(r.Context(), r.URL.Query().Get("name"), start, end)
	if err != nil {
		return err
	}

	t, _ := strconv.ParseInt(r.URL.Query().Get("total"), 10, 64)
	tpl := goatcounter.HorizontalChart(r.Context(), sizeStat, total, int(t), .2, true, false)

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) locations(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, goatcounter.MustGetSite(r.Context()))
	if err != nil {
		return err
	}

	var locStat goatcounter.Stats
	total, err := locStat.ListLocations(r.Context(), start, end)
	if err != nil {
		return err
	}

	tpl := goatcounter.HorizontalChart(r.Context(), locStat, total, total, 0, false, true)
	return zhttp.JSON(w, map[string]interface{}{
		"html": string(tpl),
	})
}

func (h backend) pages(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	start, end, err := getPeriod(w, r, site)
	if err != nil {
		return err
	}
	daily, forcedDaily := getDaily(r, start, end)

	var pages goatcounter.HitStats
	totalHits, totalUnique, totalDisplay, totalUniqueDisplay, more, err := pages.List(r.Context(), start, end,
		r.URL.Query().Get("filter"), strings.Split(r.URL.Query().Get("exclude"), ","))
	if err != nil {
		return err
	}

	tpl, err := zhttp.ExecuteTpl("_backend_pages.gohtml", struct {
		Context     context.Context
		Pages       goatcounter.HitStats
		Site        *goatcounter.Site
		PeriodStart time.Time
		PeriodEnd   time.Time
		Daily       bool
		ForcedDaily bool

		// Dummy values so template won't error out.
		Refs     bool
		ShowRefs string
	}{r.Context(), pages, goatcounter.MustGetSite(r.Context()), start, end,
		daily, forcedDaily, false, ""})
	if err != nil {
		return err
	}

	paths := make([]string, len(pages))
	for i := range pages {
		paths[i] = pages[i].Path
	}

	return zhttp.JSON(w, map[string]interface{}{
		"rows":                 string(tpl),
		"paths":                paths,
		"total_hits":           totalHits,
		"total_display":        totalDisplay,
		"total_unique":         totalUnique,
		"total_unique_display": totalUniqueDisplay,
		"more":                 more,
	})
}

func (h backend) updates(w http.ResponseWriter, r *http.Request) error {
	u := goatcounter.GetUser(r.Context())

	var up goatcounter.Updates
	err := up.List(r.Context(), u.SeenUpdatesAt)
	if err != nil {
		return err
	}

	seenat := u.SeenUpdatesAt
	err = u.SeenUpdates(r.Context())
	if err != nil {
		zlog.Field("user", fmt.Sprintf("%d", u.ID)).Error(err)
	}

	return zhttp.Template(w, "backend_updates.gohtml", struct {
		Globals
		Updates goatcounter.Updates
		SeenAt  time.Time
	}{newGlobals(w, r), up, seenat})
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

	del := map[string]interface{}{
		"ContactMe": r.URL.Query().Get("contact_me") == "true",
		"Reason":    r.URL.Query().Get("reason"),
	}

	return zhttp.Template(w, "backend_settings.gohtml", struct {
		Globals
		SubSites  goatcounter.Sites
		Validate  *zvalidate.Validator
		Timezones []*tz.Zone
		Delete    map[string]interface{}
	}{newGlobals(w, r), sites, verr, tz.Zones, del})
}

func (h backend) code(w http.ResponseWriter, r *http.Request) error {
	var sites goatcounter.Sites
	err := sites.ListSubs(r.Context())
	if err != nil {
		return err
	}

	cd := cfg.DomainCount
	if cd == "" {
		cd = goatcounter.MustGetSite(r.Context()).Domain()
		if cfg.Port != "" {
			cd += ":" + cfg.Port
		}
	}

	return zhttp.Template(w, "backend_code.gohtml", struct {
		Globals
		SubSites    goatcounter.Sites
		CountDomain string
	}{newGlobals(w, r), sites, cd})
}

func (h backend) ip(w http.ResponseWriter, r *http.Request) error {
	return zhttp.String(w, zhttp.RemovePort(r.RemoteAddr))
}

func (h backend) saveSettings(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()

	args := struct {
		Name       string                   `json:"name"`
		Cname      string                   `json:"cname"`
		LinkDomain string                   `json:"link_domain"`
		Settings   goatcounter.SiteSettings `json:"settings"`
		User       goatcounter.User         `json:"user"`
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
	if err != nil {
		return err
	}
	defer tx.Rollback()

	user := goatcounter.GetUser(txctx)

	emailChanged := false
	if cfg.Saas && args.User.Email != user.Email {
		emailChanged = true
	}

	user.Name = args.User.Name
	user.Email = args.User.Email
	err = user.Update(txctx, emailChanged)
	if err != nil {
		var vErr *zvalidate.Validator
		if !errors.As(err, &vErr) {
			return err
		}
		v.Sub("user", "", err)
	}

	site := goatcounter.MustGetSite(txctx)
	site.Name = args.Name
	site.Settings = args.Settings
	site.LinkDomain = args.LinkDomain
	if args.Cname != "" && !site.PlanCustomDomain(txctx) {
		return guru.New(http.StatusForbidden, "need a business plan to set custom domain")
	}

	makecert := false
	if args.Cname == "" {
		site.Cname = nil
	} else {
		if site.Cname == nil || *site.Cname != args.Cname {
			makecert = true // Make after we persisted to DB.
		}
		site.Cname = &args.Cname
	}

	err = site.Update(txctx)
	if err != nil {
		var vErr *zvalidate.Validator
		if !errors.As(err, &vErr) {
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

	if emailChanged {
		sendEmailVerify(site, user)
	}

	if makecert {
		go func() {
			defer zlog.Recover()

			err := acme.Make(args.Cname)
			if err != nil {
				zlog.Field("domain", args.Cname).Error(err)
			}
		}()
	}

	zhttp.Flash(w, "Saved!")
	return zhttp.SeeOther(w, "/settings")
}

func (h backend) startExport(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	ctx := goatcounter.NewContext(r.Context())

	f := goatcounter.ExportFile(site) + ".progress"
	fp, err := os.Create(f)
	if err != nil {
		return err
	}
	go goatcounter.Export(ctx, fp)

	zhttp.Flash(w, "Export started in the background; you’ll get an email with a download link when it’s done.")
	return zhttp.SeeOther(w, "/settings#tab-export")
}

func (h backend) downloadExport(w http.ResponseWriter, r *http.Request) error {
	f := goatcounter.ExportFile(goatcounter.MustGetSite(r.Context()))
	fp, err := os.Open(f)
	if err != nil {
		if os.IsNotExist(err) {
			zhttp.FlashError(w, "It looks like there is no export yet.")
			return zhttp.SeeOther(w, "/settings#tab-export")
		}

		return err
	}
	defer fp.Close()

	err = header.SetContentDisposition(w.Header(), header.DispositionArgs{
		Type:     header.TypeAttachment,
		Filename: filepath.Base(f),
	})
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/csv")
	return zhttp.Stream(w, fp)
}

func (h backend) removeSubsiteConfirm(w http.ResponseWriter, r *http.Request) error {
	if !cfg.Saas {
		return guru.New(400, "can only do this in SaaS mode")
	}

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
	if !cfg.Saas {
		return guru.New(400, "can only do this in SaaS mode")
	}

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
	if !cfg.Saas {
		return guru.New(400, "can only do this in SaaS mode")
	}

	args := struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}{}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	parent := goatcounter.MustGetSite(r.Context())
	site := goatcounter.Site{
		Code:     args.Code,
		Name:     args.Name,
		Parent:   &parent.ID,
		Plan:     goatcounter.PlanChild,
		Settings: parent.Settings,
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

func hasPlan(site *goatcounter.Site) (bool, error) {
	if !cfg.Saas || site.Plan == goatcounter.PlanChild ||
		site.Stripe == nil || site.FreePlan() || site.PayExternal() != "" {
		return false, nil
	}

	var customer struct {
		Subscriptions struct {
			Data []struct {
				CancelAtPeriodEnd bool               `json:"cancel_at_period_end"`
				CurrentPeriodEnd  jsonutil.Timestamp `json:"current_period_end"`
				Plan              struct {
					Quantity int `json:"quantity"`
				} `json:"plan"`
			} `json:"data"`
		} `json:"subscriptions"`
	}
	_, err := zstripe.Request(&customer, "GET",
		fmt.Sprintf("/v1/customers/%s", *site.Stripe), "")
	if err != nil {
		return false, err
	}

	if len(customer.Subscriptions.Data) == 0 {
		return false, nil
	}

	if customer.Subscriptions.Data[0].CancelAtPeriodEnd {
		return false, nil
	}

	return true, nil
}

func (h backend) delete(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	if cfg.Saas {
		var args struct {
			Reason    string `json:"reason"`
			ContactMe bool   `json:"contact_me"`
		}
		_, err := zhttp.Decode(r, &args)
		if err != nil {
			zlog.Error(err)
		}

		has, err := hasPlan(site)
		if err != nil {
			return err
		}
		if has {
			zhttp.FlashError(w, "This site still has a Stripe subscription; cancel that first on the billing page.")
			q := url.Values{}
			q.Set("reason", args.Reason)
			q.Set("contact_me", fmt.Sprintf("%t", args.ContactMe))
			return zhttp.SeeOther(w, "/settings?"+q.Encode()+"#tab-delete")
		}

		if args.Reason != "" {
			go func() {
				defer zlog.Recover()

				zmail.Send("GoatCounter deletion",
					mail.Address{Name: "GoatCounter deletion", Address: "support@goatcounter.com"},
					[]mail.Address{{Address: "support@goatcounter.com"}},
					fmt.Sprintf(`Deleted: %s (%d): contact_me: %t; reason: %s`,
						site.Code, site.ID, args.ContactMe, args.Reason))
			}()
		}
	}

	err := site.Delete(r.Context())
	if err != nil {
		return err
	}

	if site.Parent != nil {
		var p goatcounter.Site
		err := p.ByID(r.Context(), *site.Parent)
		if err != nil {
			return err
		}
		return zhttp.SeeOther(w, p.URL())
	}

	return zhttp.SeeOther(w, "https://"+cfg.Domain)
}

func getPeriod(w http.ResponseWriter, r *http.Request, site *goatcounter.Site) (time.Time, time.Time, error) {
	var start, end time.Time

	if d := r.URL.Query().Get("period-start"); d != "" {
		var err error
		start, err = time.ParseInLocation("2006-01-02", d, site.Settings.Timezone.Loc())
		if err != nil {
			return start, end, guru.Errorf(400, "Invalid start date: %q", d)
		}
	}
	if d := r.URL.Query().Get("period-end"); d != "" {
		var err error
		end, err = time.ParseInLocation("2006-01-02 15:04:05", d+" 23:59:59", site.Settings.Timezone.Loc())
		if err != nil {
			return start, end, guru.Errorf(400, "Invalid end date: %q", d)
		}
	}

	if start.Before(site.CreatedAt) {
		start = site.CreatedAt
	}

	return start.UTC(), end.UTC(), nil
}

func getDaily(r *http.Request, start, end time.Time) (daily bool, forced bool) {
	if end.Sub(start).Hours()/24 >= DailyView {
		return true, true
	}
	d := strings.ToLower(r.URL.Query().Get("daily"))
	return d == "on" || d == "true", false
}

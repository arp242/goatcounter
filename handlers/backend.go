// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arp242/geoip2-golang"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/monoculum/formam"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/pack"
	"zgo.at/guru"
	"zgo.at/isbot"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zhttp/mware"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zlog"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zstring"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

type backend struct{}

// DailyView forces the "view by day" if the number of selected days is larger than this.
const DailyView = 90

func (h backend) Mount(r chi.Router, db zdb.DB) {
	if !cfg.Prod {
		r.Use(mware.Delay(0))
	}

	r.Use(
		mware.RealIP(),
		mware.WrapWriter(),
		mware.Unpanic(),
		addctx(db, true),
		middleware.RedirectSlashes,
		mware.NoStore())
	if zstring.Contains(zlog.Config.Debug, "req") || zstring.Contains(zlog.Config.Debug, "all") {
		r.Use(mware.RequestLog(nil, "/count"))
	}

	api{}.mount(r, db)
	vcounter{}.mount(r, db)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(404, "Not Found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(405, "Method Not Allowed"))
	})

	{
		rr := r.With(mware.Headers(nil))
		rr.Get("/robots.txt", zhttp.HandlerRobots([][]string{{"User-agent: *", "Disallow: /"}}))
		rr.Post("/jserr", zhttp.HandlerJSErr())
		rr.Post("/csp", zhttp.HandlerCSP())

		// 4 pageviews/second should be more than enough.
		rateLimited := rr.With(mware.Ratelimit(mware.RatelimitOptions{
			Client: func(r *http.Request) string {
				// Add in the User-Agent to reduce the problem of multiple
				// people in the same building hitting the limit.
				return r.RemoteAddr + r.UserAgent()
			},
			Store: mware.NewRatelimitMemory(),
			Limit: func(r *http.Request) (int, int64) {
				if !cfg.Prod {
					return 1 << 30, 1
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
			"Strict-Transport-Security": []string{"max-age=7776000"},
			"X-Frame-Options":           []string{"deny"},
			"X-Content-Type-Options":    []string{"nosniff"},
		}

		// https://stripe.com/docs/security#content-security-policy
		ds := []string{header.CSPSourceSelf}
		if cfg.DomainStatic != "" {
			ds = append(ds, cfg.DomainStatic)
		}
		header.SetCSP(headers, header.CSPArgs{
			header.CSPDefaultSrc:  {header.CSPSourceNone},
			header.CSPImgSrc:      append(ds, "data:"),
			header.CSPScriptSrc:   append(ds, "https://chat.goatcounter.com", "https://js.stripe.com"),
			header.CSPStyleSrc:    append(ds, header.CSPSourceUnsafeInline), // style="height: " on the charts.
			header.CSPFontSrc:     ds,
			header.CSPFormAction:  {header.CSPSourceSelf, "https://explain.dalibo.com/new"},
			header.CSPConnectSrc:  {header.CSPSourceSelf, "https://chat.goatcounter.com", "https://api.stripe.com"},
			header.CSPFrameSrc:    {header.CSPSourceSelf, "https://js.stripe.com", "https://hooks.stripe.com"},
			header.CSPManifestSrc: ds,
			// Too much noise: header.CSPReportURI:  {"/csp"},
		})

		a := r.With(mware.Headers(headers), keyAuth)
		user{}.mount(a)
		{
			ap := a.With(loggedInOrPublic)
			ap.Get("/", zhttp.Wrap(h.dashboard))
			ap.Get("/pages", zhttp.Wrap(h.pages))
			ap.Get("/hchart-detail", zhttp.Wrap(h.hchartDetail))
			ap.Get("/hchart-more", zhttp.Wrap(h.hchartMore))
		}
		{
			af := a.With(loggedIn)
			if zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "" {
				billing{}.mount(a, af)
			}
			af.Get("/updates", zhttp.Wrap(h.updates))
			af.Get("/code", zhttp.Wrap(h.code))

			settings{}.mount(af)
			admin{}.mount(af, db)
		}
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

func (h backend) count(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "image/gif")

	// Note this works in both HTTP/1.1 and HTTP/2, as the Go HTTP/2 server
	// picks up on this and sends the GOAWAY frame.
	// TODO: it would be better to set a short idle timeout, but this isn't
	// really something that can be configured per-handler at the moment.
	// https://github.com/golang/go/issues/16100
	w.Header().Set("Connection", "close")

	bot := isbot.Bot(r)
	// Don't track pages fetched with the browser's prefetch algorithm.
	if bot == isbot.BotPrefetch {
		return zhttp.Bytes(w, gif)
	}

	site := Site(r.Context())
	for _, ip := range site.Settings.IgnoreIPs {
		if ip == r.RemoteAddr {
			w.Header().Add("X-Goatcounter", fmt.Sprintf("ignored because %q is in the IP ignore list", ip))
			w.WriteHeader(http.StatusAccepted)
			return zhttp.Bytes(w, gif)
		}
	}

	hit := goatcounter.Hit{
		Site:            site.ID,
		UserAgentHeader: r.UserAgent(),
		Location:        geo(r.RemoteAddr),
		CreatedAt:       goatcounter.Now(),
		RemoteAddr:      r.RemoteAddr,
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

	err = hit.Validate(r.Context(), true)
	if err != nil {
		w.Header().Add("X-Goatcounter", fmt.Sprintf("not valid: %s", err))
		w.WriteHeader(400)
		return zhttp.Bytes(w, gif)
	}

	goatcounter.Memstore.Append(hit)
	return zhttp.Bytes(w, gif)
}

func (h backend) pages(w http.ResponseWriter, r *http.Request) error {
	site := Site(r.Context())

	exclude, err := zint.Split(r.URL.Query().Get("exclude"), ",")
	if err != nil {
		return err
	}

	var (
		filter     = r.URL.Query().Get("filter")
		pathFilter []int64
	)
	if filter != "" {
		pathFilter, err = goatcounter.PathFilter(r.Context(), filter, true)
		if err != nil {
			return err
		}
	}

	asText := r.URL.Query().Get("as-text") == "on" || r.URL.Query().Get("as-text") == "true"
	start, end, err := getPeriod(w, r, site)
	if err != nil {
		return err
	}
	daily, forcedDaily := getDaily(r, start, end)
	m, err := strconv.ParseInt(r.URL.Query().Get("max"), 10, 64)
	if err != nil {
		return err
	}
	max := int(m)

	o, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil {
		o = 1
	}
	offset := int(o)

	// Load new totals unless this is for pagination.
	var (
		wg sync.WaitGroup

		totalTpl   string
		totalPages goatcounter.HitStat
		totalErr   error

		maxTotals int
		maxErr    error

		totalHits, totalUnique int
		totalCountErr          error
	)

	// Filtering instead of paginating: get new "totals" stats as well.
	// TODO: also re-render the the horizontal bar charts below, but this isn't
	// currently possible since not all data is linked to a path.
	//
	// TODO: use widgets for this.
	if len(exclude) == 0 {
		wg.Add(1)
		go func() {
			defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.FieldsRequest(r) })
			defer wg.Done()

			maxTotals, totalErr = totalPages.Totals(r.Context(), start, end, pathFilter, daily)
			if totalErr != nil {
				return
			}

			totalTpl, totalErr = ztpl.ExecuteString("_dashboard_totals_row.gohtml", struct {
				Context context.Context
				Site    *goatcounter.Site
				Page    goatcounter.HitStat
				Daily   bool
				Max     int
			}{r.Context(), site, totalPages, daily, maxTotals})
		}()

		wg.Add(1)
		go func() {
			defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.FieldsRequest(r) })
			defer wg.Done()

			max, maxErr = goatcounter.GetMax(r.Context(), start, end, pathFilter, daily)
		}()

		wg.Add(1)
		go func() {
			defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.FieldsRequest(r) })
			defer wg.Done()

			totalHits, totalUnique, _, totalCountErr = goatcounter.GetTotalCount(r.Context(), start, end, pathFilter)
		}()
	}

	var pages goatcounter.HitStats
	totalDisplay, totalUniqueDisplay, more, err := pages.List(
		r.Context(), start, end, pathFilter, exclude, daily)
	if err != nil {
		return err
	}

	t := "_dashboard_pages_rows.gohtml"
	if asText {
		t = "_dashboard_pages_text_rows.gohtml"
	}

	tpl, err := ztpl.ExecuteString(t, struct {
		Context     context.Context
		Pages       goatcounter.HitStats
		Site        *goatcounter.Site
		PeriodStart time.Time
		PeriodEnd   time.Time
		Daily       bool
		ForcedDaily bool
		Max         int
		Offset      int

		// Dummy values so template won't error out.
		Refs     bool
		ShowRefs string
	}{r.Context(), pages, site, start, end, daily, forcedDaily, int(max),
		offset, false, ""})
	if err != nil {
		return err
	}

	paths := make([]string, len(pages))
	for i := range pages {
		paths[i] = pages[i].Path
	}

	wg.Wait()
	if totalErr != nil {
		return totalErr
	}
	if maxErr != nil {
		return maxErr
	}
	if totalCountErr != nil {
		return totalCountErr
	}

	return zhttp.JSON(w, map[string]interface{}{
		"rows":                 tpl,
		"totals":               totalTpl,
		"paths":                paths,
		"total_hits":           totalHits,
		"total_display":        totalDisplay,
		"total_unique":         totalUnique,
		"total_unique_display": totalUniqueDisplay,
		"max":                  max,
		"more":                 more,
	})
}

// TODO: don't hard-code limit to 10, and allow pagination here too.
func (h backend) hchartDetail(w http.ResponseWriter, r *http.Request) error {
	start, end, err := getPeriod(w, r, Site(r.Context()))
	if err != nil {
		return err
	}

	v := zvalidate.New()
	name := r.URL.Query().Get("name")
	kind := r.URL.Query().Get("kind")
	v.Required("name", name)
	v.Include("kind", kind, []string{"browser", "system", "size", "topref"})
	v.Required("kind", kind)
	total := int(v.Integer("total", r.URL.Query().Get("total")))
	if v.HasErrors() {
		return v
	}

	var (
		filter     = r.URL.Query().Get("filter")
		pathFilter []int64
	)
	if filter != "" {
		pathFilter, err = goatcounter.PathFilter(r.Context(), filter, true)
		if err != nil {
			return err
		}
	}

	var detail goatcounter.Stats
	switch kind {
	case "browser":
		err = detail.ListBrowser(r.Context(), name, start, end, pathFilter)
	case "system":
		err = detail.ListSystem(r.Context(), name, start, end, pathFilter)
	case "size":
		err = detail.ListSize(r.Context(), name, start, end, pathFilter)
	case "topref":
		if name == "(unknown)" {
			name = ""
		}
		err = detail.ByRef(r.Context(), start, end, pathFilter, name)
	}
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(goatcounter.HorizontalChart(r.Context(), detail, total, 10, false, false)),
	})
}

func (h backend) hchartMore(w http.ResponseWriter, r *http.Request) error {
	site := Site(r.Context())

	start, end, err := getPeriod(w, r, site)
	if err != nil {
		return err
	}

	v := zvalidate.New()
	kind := r.URL.Query().Get("kind")
	v.Include("kind", kind, []string{"browser", "system", "location", "ref", "topref"})
	v.Required("kind", kind)
	total := int(v.Integer("total", r.URL.Query().Get("total")))
	offset := int(v.Integer("offset", r.URL.Query().Get("offset")))

	var (
		filter     = r.URL.Query().Get("filter")
		pathFilter []int64
	)
	if filter != "" {
		pathFilter, err = goatcounter.PathFilter(r.Context(), filter, true)
		if err != nil {
			return err
		}
	}

	showRefs := ""
	if kind == "ref" {
		showRefs = r.URL.Query().Get("showrefs")
		v.Required("showrefs", "showRefs")
	}
	if v.HasErrors() {
		return v
	}

	var (
		page     goatcounter.Stats
		size     = 6
		paginate = false
		link     = true
	)
	switch kind {
	case "browser":
		err = page.ListBrowsers(r.Context(), start, end, pathFilter, 6, offset)
	case "system":
		err = page.ListSystems(r.Context(), start, end, pathFilter, 6, offset)
	case "location":
		err = page.ListLocations(r.Context(), start, end, pathFilter, 6, offset)
		link = false
	case "ref":
		err = page.ListRefsByPath(r.Context(), showRefs, start, end, offset)
		size = site.Settings.LimitRefs()
		paginate = offset == 0
		link = false
	case "topref":
		err = page.ListTopRefs(r.Context(), start, end, pathFilter, offset)
	}
	if err != nil {
		return err
	}

	return zhttp.JSON(w, map[string]interface{}{
		"html": string(goatcounter.HorizontalChart(r.Context(), page, total, size, link, paginate)),
		"more": page.More,
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

func (h backend) code(w http.ResponseWriter, r *http.Request) error {
	var sites goatcounter.Sites
	err := sites.ListSubs(r.Context())
	if err != nil {
		return err
	}

	cd := cfg.DomainCount
	if cd == "" {
		cd = Site(r.Context()).Domain()
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

func hasPlan(site *goatcounter.Site) (bool, error) {
	if !cfg.GoatcounterCom || site.Plan == goatcounter.PlanChild ||
		site.Stripe == nil || *site.Stripe == "" || site.FreePlan() || site.PayExternal() != "" {
		return false, nil
	}

	var customer struct {
		Subscriptions struct {
			Data []struct {
				CancelAtPeriodEnd bool            `json:"cancel_at_period_end"`
				CurrentPeriodEnd  zjson.Timestamp `json:"current_period_end"`
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

	// Allow viewing a week before the site was created at the most.
	c := site.FirstHitAt.Add(-24 * time.Hour * 7)
	if start.Before(c) {
		y, m, d := c.In(site.Settings.Timezone.Loc()).Date()
		start = time.Date(y, m, d, 0, 0, 0, 0, site.Settings.Timezone.Loc())
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

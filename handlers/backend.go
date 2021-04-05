// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"zgo.at/goatcounter"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/header"
	"zgo.at/zhttp/mware"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztime"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

// DailyView forces the "view by day" if the number of selected days is larger than this.
const DailyView = 90

func NewBackend(db zdb.DB, acmeh http.HandlerFunc, dev, goatcounterCom bool, domainStatic string, dashTimeout int) chi.Router {
	r := chi.NewRouter()
	backend{dashTimeout}.Mount(r, db, dev, domainStatic, dashTimeout)

	if acmeh != nil {
		r.Get("/.well-known/acme-challenge/{key}", acmeh)
	}

	if !goatcounterCom {
		NewStatic(r, dev)
	}

	return r
}

type backend struct{ dashTimeout int }

func (h backend) Mount(r chi.Router, db zdb.DB, dev bool, domainStatic string, dashTimeout int) {
	if dev {
		r.Use(mware.Delay(0))
	}

	r.Use(
		mware.RealIP(),
		mware.WrapWriter(),
		mware.Unpanic(),
		addctx(db, true, dashTimeout),
		middleware.RedirectSlashes,
		mware.NoStore())
	if zstring.Contains(zlog.Config.Debug, "req") || zstring.Contains(zlog.Config.Debug, "all") {
		r.Use(mware.RequestLog(nil, "/count"))
	}

	fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "", dev)
	if err != nil {
		panic(err)
	}
	static, err := zfs.EmbedOrDir(goatcounter.Static, "public", dev)
	if err != nil {
		panic(err)
	}

	website{fsys, false, ""}.MountShared(r)
	api{}.mount(r, db)
	vcounter{static}.mount(r)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(404, "Not Found"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(405, "Method Not Allowed"))
	})

	{
		rr := r.With(mware.Headers(nil))
		rr.Get("/robots.txt", zhttp.HandlerRobots([][]string{{"User-agent: *", "Disallow: /"}}))
		rr.Get("/security.txt", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return zhttp.Text(w, "Contact: support@goatcounter.com")
		}))
		rr.Post("/jserr", zhttp.HandlerJSErr())
		rr.Post("/csp", zhttp.HandlerCSP())

		// 4 pageviews/second should be more than enough.
		rate := rr.With(mware.Ratelimit(mware.RatelimitOptions{
			Client: func(r *http.Request) string {
				// Add in the User-Agent to reduce the problem of multiple
				// people in the same building hitting the limit.
				return r.RemoteAddr + r.UserAgent()
			},
			Store: mware.NewRatelimitMemory(),
			Limit: func(r *http.Request) (int, int64) {
				if dev {
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
		rate.Get("/count", zhttp.Wrap(h.count))
		rate.Post("/count", zhttp.Wrap(h.count)) // to support navigator.sendBeacon (JS)
	}

	{
		headers := http.Header{
			"Strict-Transport-Security": []string{"max-age=7776000"},
			"X-Frame-Options":           []string{"deny"},
			"X-Content-Type-Options":    []string{"nosniff"},
		}

		// https://stripe.com/docs/security#content-security-policy
		ds := []string{header.CSPSourceSelf}
		if domainStatic != "" {
			ds = append(ds, domainStatic)
		}
		header.SetCSP(headers, header.CSPArgs{
			header.CSPDefaultSrc:  {header.CSPSourceNone},
			header.CSPImgSrc:      append(ds, "data:"),
			header.CSPScriptSrc:   append(ds, "https://js.stripe.com"),
			header.CSPStyleSrc:    append(ds, header.CSPSourceUnsafeInline), // style="height: " on the charts.
			header.CSPFontSrc:     ds,
			header.CSPFormAction:  {header.CSPSourceSelf, "https://billing.stripe.com"},
			header.CSPConnectSrc:  {header.CSPSourceSelf, "https://api.stripe.com"},
			header.CSPFrameSrc:    {header.CSPSourceSelf, "https://js.stripe.com", "https://hooks.stripe.com"},
			header.CSPManifestSrc: ds,
			// Too much noise: header.CSPReportURI:  {"/csp"},
		})

		a := r.With(mware.Headers(headers), keyAuth)
		user{}.mount(a)
		{
			ap := a.With(loggedInOrPublic)
			ap.Get("/", zhttp.Wrap(h.dashboard))
			ap.Get("/pages-more", zhttp.Wrap(h.pagesMore))
			ap.Get("/hchart-detail", zhttp.Wrap(h.hchartDetail))
			ap.Get("/hchart-more", zhttp.Wrap(h.hchartMore))
		}
		{
			af := a.With(loggedIn)
			if zstripe.SecretKey != "" && zstripe.SignSecret != "" && zstripe.PublicKey != "" {
				billing{}.mount(a, af)
			}
			af.Get("/updates", zhttp.Wrap(h.updates))

			settings{}.mount(af)
			bosmang{}.mount(af, db)
		}
	}
}

func (h backend) pagesMore(w http.ResponseWriter, r *http.Request) error {
	site := Site(r.Context())
	user := User(r.Context())

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
	rng, err := getPeriod(w, r, site, user)
	if err != nil {
		return err
	}
	daily, forcedDaily := getDaily(r, rng)
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

	var pages goatcounter.HitLists
	totalDisplay, totalUniqueDisplay, more, err := pages.List(
		r.Context(), rng, pathFilter, exclude, daily)
	if err != nil {
		return err
	}

	t := "_dashboard_pages_rows.gohtml"
	if asText {
		t = "_dashboard_pages_text_rows.gohtml"
	}
	tpl, err := ztpl.ExecuteString(t, struct {
		Globals
		Pages       goatcounter.HitLists
		Period      ztime.Range
		Daily       bool
		ForcedDaily bool
		Max         int
		Offset      int

		// Dummy values so template won't error out.
		Refs     bool
		ShowRefs string
	}{newGlobals(w, r), pages, rng, daily, forcedDaily, int(max),
		offset, false, ""})
	if err != nil {
		return err
	}

	paths := make([]string, len(pages))
	for i := range pages {
		paths[i] = pages[i].Path
	}

	return zhttp.JSON(w, map[string]interface{}{
		"rows":                 tpl,
		"paths":                paths,
		"total_display":        totalDisplay,
		"total_unique_display": totalUniqueDisplay,
		"max":                  max,
		"more":                 more,
	})
}

// TODO: don't hard-code limit to 10, and allow pagination here too.
func (h backend) hchartDetail(w http.ResponseWriter, r *http.Request) error {
	site := Site(r.Context())
	user := User(r.Context())
	rng, err := getPeriod(w, r, site, user)
	if err != nil {
		return err
	}

	v := zvalidate.New()
	name := r.URL.Query().Get("name")
	kind := r.URL.Query().Get("kind")
	v.Required("name", name)
	v.Include("kind", kind, []string{"browser", "system", "size", "topref", "location"})
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

	var detail goatcounter.HitStats
	switch kind {
	case "browser":
		err = detail.ListBrowser(r.Context(), name, rng, pathFilter)
	case "system":
		err = detail.ListSystem(r.Context(), name, rng, pathFilter)
	case "size":
		err = detail.ListSize(r.Context(), name, rng, pathFilter)
	case "location":
		err = detail.ListLocation(r.Context(), name, rng, pathFilter)
	case "topref":
		if name == "(unknown)" {
			name = ""
		}
		err = detail.ByRef(r.Context(), rng, pathFilter, name)
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
	user := User(r.Context())

	rng, err := getPeriod(w, r, site, user)
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
		page     goatcounter.HitStats
		size     = 6
		paginate = false
		link     = true
	)
	switch kind {
	case "browser":
		err = page.ListBrowsers(r.Context(), rng, pathFilter, 6, offset)
	case "system":
		err = page.ListSystems(r.Context(), rng, pathFilter, 6, offset)
	case "location":
		err = page.ListLocations(r.Context(), rng, pathFilter, 6, offset)
	case "ref":
		err = page.ListRefsByPath(r.Context(), showRefs, rng, offset)
		size = user.Settings.LimitRefs()
		paginate = offset == 0
		link = false
	case "topref":
		err = page.ListTopRefs(r.Context(), rng, pathFilter, offset)
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
	u := User(r.Context())

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

func hasPlan(ctx context.Context, site *goatcounter.Site) (bool, error) {
	if !goatcounter.Config(ctx).GoatcounterCom || site.Plan == goatcounter.PlanChild ||
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

func getPeriod(w http.ResponseWriter, r *http.Request, site *goatcounter.Site, user *goatcounter.User) (ztime.Range, error) {
	var rng ztime.Range

	if d := r.URL.Query().Get("period-start"); d != "" {
		var err error
		rng.Start, err = time.ParseInLocation("2006-01-02", d, user.Settings.Timezone.Loc())
		if err != nil {
			return rng, guru.Errorf(400, "Invalid start date: %q", d)
		}
	}
	if d := r.URL.Query().Get("period-end"); d != "" {
		var err error
		rng.End, err = time.ParseInLocation("2006-01-02 15:04:05", d+" 23:59:59", user.Settings.Timezone.Loc())
		if err != nil {
			return rng, guru.Errorf(400, "Invalid end date: %q", d)
		}
	}

	// Allow viewing a week before the site was created at the most.
	c := site.FirstHitAt.Add(-24 * time.Hour * 7)
	if rng.Start.Before(c) {
		y, m, d := c.In(user.Settings.Timezone.Loc()).Date()
		rng.Start = time.Date(y, m, d, 0, 0, 0, 0, user.Settings.Timezone.Loc())
	}

	return rng.From(rng.Start).To(rng.End).UTC(), nil
}

func getDaily(r *http.Request, rng ztime.Range) (daily bool, forced bool) {
	if rng.End.Sub(rng.Start).Hours()/24 >= DailyView {
		return true, true
	}
	d := strings.ToLower(r.URL.Query().Get("daily"))
	return d == "on" || d == "true", false
}

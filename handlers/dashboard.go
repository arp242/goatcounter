package handlers

import (
	"context"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/goatcounter/v2/pkg/metrics"
	"zgo.at/goatcounter/v2/widgets"
	"zgo.at/guru"
	"zgo.at/z18n"
	"zgo.at/zhttp"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zstrconv"
	"zgo.at/zstd/zsync"
	"zgo.at/zstd/ztime"
	"zgo.at/ztpl"
	"zgo.at/zvalidate"
)

func (h backend) dashboard(w http.ResponseWriter, r *http.Request) error {
	m := metrics.Start(r.Host)
	defer m.Done()

	site := Site(r.Context())
	user := User(r.Context())

	// Cache much more aggressively for public displays. Don't care so much if
	// it's outdated by an hour.
	if site.Settings.IsPublic() && User(r.Context()).ID == 0 {
		w.Header().Set("Cache-Control", "public,max-age=3600")
		w.Header().Set("Vary", "Cookie")
	}

	q := r.URL.Query()

	// Load view, but override this from query.
	view, _ := user.Settings.Views.Get("default")

	rng, err := getPeriod(w, r, site, user)
	if err != nil {
		zhttp.FlashError(w, r, err.Error())
	}
	if rng.Start.IsZero() || rng.End.IsZero() {
		rng = timeRange(r.Context(), view.Period, user.Settings.Timezone.Loc(), bool(user.Settings.SundayStartsWeek))
		if err != nil {
			return err
		}
	} else {
		view.Period = q.Get("hl-period")
	}
	// Record how often people use the "Current [..]" buttons
	if strings.HasSuffix(view.Period, "-cur") {
		metrics.Start(view.Period).Done()
	}

	showRefs, _ := zstrconv.ParseInt[goatcounter.PathID](q.Get("showrefs"), 10)
	if _, ok := q["filter"]; ok {
		view.Filter = q.Get("filter")
	}
	var forcedGroup bool
	view.Group, forcedGroup = getGroup(r, rng)

	// Get path IDs to filter first, as they're used by the widgets.
	var (
		pathFilter = make(chan (struct {
			Paths []goatcounter.PathID
			Err   error
		}))
	)
	go func() {
		defer log.Recover(r.Context(), func(err error) { log.Error(r.Context(), err, "filter", view.Filter, log.AttrHTTP(r)) })

		var (
			f     []goatcounter.PathID
			start = ztime.Now(r.Context())
			err   error
		)
		if view.Filter != "" {
			f, err = goatcounter.PathFilter(r.Context(), view.Filter, true)
		}
		pathFilter <- struct {
			Paths []goatcounter.PathID
			Err   error
		}{f, err}
		log.Module("dashboard").Debug(r.Context(), "pathfilter",
			"took", time.Since(start).Round(time.Millisecond))
	}()

	subs, err := site.ListSubs(r.Context())
	if err != nil {
		return err
	}

	cd := goatcounter.Config(r.Context()).DomainCount
	if cd == "" {
		cd = Site(r.Context()).SchemelessURL(r.Context())
	}

	args := widgets.Args{
		Rng:         rng,
		Group:       view.Group,
		ForcedGroup: forcedGroup,
		ShowRefs:    showRefs,
	}

	f := <-pathFilter
	args.PathFilter, err = f.Paths, f.Err
	if err != nil {
		return err
	}

	// Load widgets data from the database.
	wid := widgets.FromSiteWidgets(r.Context(), user.Settings.Widgets, 0)
	shared := widgets.SharedData{Args: args, Site: site, User: user}

	for _, w := range wid.Get("totalpages") {
		wid.GetOne("totalcount").(*widgets.TotalCount).NoEvents = w.Settings()["no-events"].Value.(bool)
	}

	initial := wid
	var lazy widgets.List
	if useWebsocket(r) {
		initial, lazy = wid.InitialAndLazy()
	}

	getData := func(w widgets.Widget, start time.Time) {
		// Create context for every goroutine, so we know which timed out.
		ctx, cancel := context.WithTimeout(goatcounter.CopyContextValues(r.Context()),
			time.Duration(h.dashTimeout)*time.Second)
		defer cancel()

		l := log.Module("dashboard")
		_, err := w.GetData(ctx, args)
		if err != nil {
			l.Error(ctx, err, log.AttrHTTP(r))
			_, err = zhttp.UserError(err)
			w.SetErr(err)
		}
		l.Debug(r.Context(), w.Name(), "took", time.Since(start))
	}
	getHTML := func(w widgets.Widget) {
		tplName, tplData := w.RenderHTML(r.Context(), shared)
		if tplName == "" { // Some data doesn't have a template.
			return
		}
		tpl, err := ztpl.ExecuteString(tplName, tplData)
		if err != nil {
			log.Module("dashboard").Error(r.Context(), err, log.AttrHTTP(r))
			w.SetHTML(template.HTML("template rendering error: " + template.HTMLEscapeString(err.Error())))
			return
		}

		w.SetHTML(template.HTML(tpl))
	}

	func() {
		var wg sync.WaitGroup
		for _, w := range initial {
			wg.Add(1)
			go func(w widgets.Widget) {
				defer wg.Done()
				defer log.Recover(r.Context(), func(err error) { log.Error(r.Context(), err, "data widget", w, log.AttrHTTP(r)) })
				getData(w, ztime.Now(r.Context()))
			}(w)
		}
		zsync.Wait(r.Context(), &wg)
	}()

	// Set shared params.
	tc := wid.GetOne("totalcount").(*widgets.TotalCount)
	shared.Total, shared.TotalUTC, shared.TotalEvents = tc.Total, tc.TotalUTC, tc.TotalEvents

	// Render widget templates.
	func() {
		var wg sync.WaitGroup
		for _, w := range wid {
			wg.Add(1)
			go func(w widgets.Widget) {
				defer log.Recover(r.Context(), func(err error) { log.Error(r.Context(), err, "tpl widget", w, log.AttrHTTP(r)) })
				defer wg.Done()

				getHTML(w)
			}(w)
		}
		zsync.Wait(r.Context(), &wg)
	}()

	// Load the rest in the background and send over websocket.
	var connectID zint.Uint128
	if c := q.Get("connectID"); c != "" {
		var err error
		connectID, err = zint.ParseUint128(c, 16)
		if err != nil {
			return err
		}
	} else {
		connectID = goatcounter.UUID()
		loader.register(connectID)
	}

	go func() {
		defer log.Recover(r.Context())
		run := zsync.NewAtMost(2)
		for _, w := range lazy {
			func(w widgets.Widget) {
				run.Run(func() {
					getData(w, ztime.Now(r.Context()))
					getHTML(w)
					loader.sendJSON(r, connectID, map[string]any{
						"id":   w.ID(),
						"html": w.HTML(),
					})
				})
			}(w)
		}
		run.Wait()
	}()

	rng = rng.In(user.Settings.Timezone.Loc()).Locale(ztime.RangeLocale{
		Today:     func() string { return T(r.Context(), "dashboard/today|Today") },
		Yesterday: func() string { return T(r.Context(), "dashboard/yesterday|Yesterday") },
		DayAgo:    func(n int) string { return T(r.Context(), "dashboard/day-ago|%(n) days ago", z18n.Plural(n)) },
		WeekAgo:   func(n int) string { return T(r.Context(), "dashboard/week-ago|%(n) weeks ago", z18n.Plural(n)) },
		MonthAgo:  func(n int) string { return T(r.Context(), "dashboard/month-ago|%(n) months ago", z18n.Plural(n)) },
		Month: func(m time.Month) string {
			return z18n.Get(r.Context()).MonthName(time.Date(0, m, 0, 0, 0, 0, 0, time.UTC), z18n.TimeFormatFull)
		},
	})

	// When reloading the dashboard from e.g. the filter we don't need to render
	// header/footer/menu, etc. Render just the widgets and return that as JSON.
	if q.Get("reload") != "" {
		t, err := ztpl.ExecuteString("_dashboard_widgets.gohtml", struct {
			Globals
			Widgets widgets.List
		}{newGlobals(w, r), wid})
		if err != nil {
			return err
		}

		return zhttp.JSON(w, map[string]string{
			"widgets":   t,
			"timerange": rng.String(),
		})
	}

	return zhttp.Template(w, "dashboard.gohtml", struct {
		Globals
		CountDomain string
		SubSites    []string
		ShowRefs    goatcounter.PathID
		Period      ztime.Range
		PathFilter  []goatcounter.PathID
		ForcedGroup bool
		Widgets     widgets.List
		View        goatcounter.View
		Total       int
		TotalUTC    int
		ConnectID   zint.Uint128
	}{newGlobals(w, r), cd, subs, showRefs, rng,
		args.PathFilter, forcedGroup, wid, view, shared.Total, shared.TotalUTC,
		connectID})
}

func (h backend) loadWidget(w http.ResponseWriter, r *http.Request) error {
	user := User(r.Context())
	rng, err := getPeriod(w, r, Site(r.Context()), user)
	if err != nil {
		return err
	}

	v := goatcounter.NewValidate(r.Context())
	var (
		widget     = int(v.Integer("widget", r.URL.Query().Get("widget")))
		key        = r.URL.Query().Get("key")
		total      = int(v.Integer("total", r.URL.Query().Get("total")))
		offset     = int(v.Integer("offset", r.URL.Query().Get("offset")))
		pathFilter = getPathFilter(&v, r)
	)
	if v.HasErrors() {
		return v
	}

	args := widgets.SharedData{
		Site:     Site(r.Context()),
		User:     User(r.Context()),
		TotalUTC: total,
		Total:    total,
		RowsOnly: key != "" || offset > 0,
		Args: widgets.Args{
			Rng:        rng,
			PathFilter: pathFilter,
			Offset:     offset,
		},
	}

	wid := widgets.FromSiteWidget(r.Context(), user.Settings.Widgets[widget])
	if key != "" {
		s := wid.Settings()
		s.Set("key", key)
		wid.SetSettings(s)
	}

	ret := make(map[string]any)
	switch wid.Name() {
	case "pages":
		p := wid.(*widgets.Pages)

		args.RowsOnly = true
		args.Args.Group, args.Args.ForcedGroup = getGroup(r, rng)

		if key != "" {
			p.RefsForPath, _ = zstrconv.ParseInt[goatcounter.PathID](key, 10)
		} else {
			p.Max, err = strconv.Atoi(r.URL.Query().Get("max"))
			if err != nil {
				return guru.Errorf(400, `"max" query parameter wrong: %w`, err)
			}
			p.Exclude, err = zint.Split[goatcounter.PathID](r.URL.Query().Get("exclude"), ",")
			if err != nil {
				return guru.Errorf(400, `"exclude" query parameter wrong: %w`, err)
			}
		}
	}

	ret["more"], err = wid.GetData(r.Context(), args.Args)
	if err != nil {
		return err
	}
	ret["html"], err = ztpl.ExecuteString(wid.RenderHTML(r.Context(), args))
	if err != nil {
		return err
	}
	switch wid.Name() {
	case "pages":
		p := wid.(*widgets.Pages)
		ret["total_display"] = p.Display
		ret["max"] = p.Max
	}

	return zhttp.JSON(w, ret)
}

// Get a time range; the return value is always in UTC, and is the UTC day range
// corresponding to the given timezone.
//
// So, for example a week in +08:00 would be:
// 2020-12-20 16:00:00 - 2020-12-27 15:59:59
//
// Values for rng:
//
//	week, month, quarter, half-year, year
//	   The start date is set to exactly this period ago. The end date is set to
//	   the end of the current day.
//
//	week-cur, month-cur, year-cur
//	   The current week or month; both the start and return are modified.
//
//	Any digit
//	   Last n days.
func timeRange(ctx context.Context, r string, tz *time.Location, sundayStartsWeek bool) ztime.Range {
	rng := ztime.NewRange(ztime.Now(ctx).In(tz)).Current(ztime.Day)
	switch r {
	case "0", "day":
	case "week-cur":
		rng = rng.Current(ztime.Week(sundayStartsWeek))
	case "month-cur":
		rng = rng.Current(ztime.Month)
	case "year-cur":
		rng = rng.Current(ztime.Year)
	case "week":
		rng = rng.Last(ztime.Week(sundayStartsWeek))
	case "month":
		rng = rng.Last(ztime.Month)
	case "quarter":
		rng = rng.Last(ztime.Quarter)
	case "half-year":
		rng = rng.Last(ztime.HalfYear)
	case "year":
		rng = rng.Last(ztime.Year)
	default:
		// Sometimes the frontend sends something like "54.958333333333336"
		// presumably due to to JS numbers always being a float. Not entirely
		// sure where this happens, so just deal with it here.
		days, err := strconv.ParseFloat(r, 32)
		if err != nil {
			log.Error(ctx, errors.Errorf("timeRange: %w", err), "rng", r)
			return timeRange(ctx, "week", tz, sundayStartsWeek)
		}
		rng.Start = ztime.AddPeriod(rng.Start, -int(math.Round(days)), ztime.Day)
	}
	return rng.UTC()
}

func getPeriod(w http.ResponseWriter, r *http.Request, site *goatcounter.Site, user *goatcounter.User) (ztime.Range, error) {
	var rng ztime.Range

	if d := r.URL.Query().Get("period-start"); d != "" {
		var err error
		rng.Start, err = time.ParseInLocation("2006-01-02", d, user.Settings.Timezone.Loc())
		if err != nil {
			return rng, guru.New(400, T(r.Context(), "error/invalid-start-date|Invalid start date: %(date)", d))
		}
	}
	if d := r.URL.Query().Get("period-end"); d != "" {
		var err error
		rng.End, err = time.ParseInLocation("2006-01-02 15:04:05", d+" 23:59:59", user.Settings.Timezone.Loc())
		if err != nil {
			return rng, guru.New(400, T(r.Context(), "error/invalid-end-date|Invalid end date: %(date)", d))
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

func getGroup(r *http.Request, rng ztime.Range) (g goatcounter.Group, forced bool) {
	// Force daily view for large timespans and force hourly for very short
	// ones, as it looks horrible otherwise.
	if d := rng.End.Sub(rng.Start).Hours() / 24; d >= 90 {
		return goatcounter.GroupDaily, true
	} else if d <= 6 {
		return goatcounter.GroupHourly, true
	}
	switch strings.ToLower(r.URL.Query().Get("group")) {
	case "day":
		return goatcounter.GroupDaily, false
	}
	return goatcounter.GroupHourly, false
}

func getPathFilter(v *zvalidate.Validator, r *http.Request) []goatcounter.PathID {
	f := r.URL.Query().Get("filter")
	if f == "" {
		return nil
	}

	filter, err := goatcounter.PathFilter(r.Context(), f, true)
	if err != nil {
		v.Append("filter", err.Error())
	}
	return filter
}

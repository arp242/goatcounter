// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"html/template"
	"net/http"
	"strconv"
	"sync"
	"time"

	nnow "github.com/jinzhu/now"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/widgets"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zhttp/ztpl/tplfunc"
	"zgo.at/zlog"
	"zgo.at/zstd/zsync"
)

const day = 24 * time.Hour

func (h backend) dashboard(w http.ResponseWriter, r *http.Request) error {
	site := Site(r.Context())

	// Cache much more aggressively for public displays. Don't care so much if
	// it's outdated by an hour.
	if site.Settings.Public && goatcounter.GetUser(r.Context()).ID == 0 {
		w.Header().Set("Cache-Control", "public,max-age=3600")
		w.Header().Set("Vary", "Cookie")
	}

	q := r.URL.Query()

	// Load view, but override this from query.
	view, _ := site.Settings.Views.Get("default")

	start, end, err := getPeriod(w, r, site)
	if err != nil {
		zhttp.FlashError(w, err.Error())
	}
	if start.IsZero() || end.IsZero() {
		start, end, err = timeRange(view.Period, site.Settings.Timezone.Loc(), site.Settings.SundayStartsWeek)
		if err != nil {
			return err
		}
	} else {
		view.Period = q.Get("hl-period")
	}

	showRefs := q.Get("showrefs")
	if _, ok := q["filter"]; ok {
		view.Filter = q.Get("filter")
	}
	if _, ok := q["as-text"]; ok {
		view.AsText = q.Get("as-text") == "on" || q.Get("as-text") == "true"
	}
	if _, ok := q["daily"]; ok {
		view.Daily = q.Get("daily") == "on" || q.Get("daily") == "true"
	}
	_, forcedDaily := getDaily(r, start, end)
	if forcedDaily {
		view.Daily = true
	}

	// Get path IDs to filter first, as they're used by the widgets.
	var (
		pathFilter = make(chan (struct {
			Paths []int64
			Err   error
		}))
	)
	go func() {
		defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.Field("filter", view.Filter).FieldsRequest(r) })

		l := zlog.Module("dashboard")

		var (
			f   []int64
			err error
		)
		if view.Filter != "" {
			f, err = goatcounter.PathFilter(r.Context(), view.Filter, true)
		}
		pathFilter <- struct {
			Paths []int64
			Err   error
		}{f, err}
		l.Since("pathfilter")
	}()

	subs, err := site.ListSubs(r.Context())
	if err != nil {
		return err
	}

	cd := goatcounter.Config(r.Context()).DomainCount
	if cd == "" {
		cd = Site(r.Context()).Domain(r.Context())
		if goatcounter.Config(r.Context()).Port != "" {
			cd += ":" + goatcounter.Config(r.Context()).Port
		}
	}

	args := widgets.Args{
		Start:       start,
		End:         end,
		Daily:       view.Daily,
		ShowRefs:    showRefs,
		ForcedDaily: forcedDaily,
		AsText:      view.AsText,
	}

	f := <-pathFilter
	args.PathFilter, err = f.Paths, f.Err
	if err != nil {
		return err
	}

	// Load widgets data from the database.
	params := widgets.FilterOff
	if showRefs != "" {
		params |= widgets.ShowRefs
	}
	wid := widgets.FromSiteWidgets(site.Settings.Widgets, params)

	func() {
		var wg sync.WaitGroup
		for _, w := range wid {
			wg.Add(1)
			go func(w widgets.Widget) {
				defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.Field("data widget", w).FieldsRequest(r) })
				defer wg.Done()

				// Create context for every goroutine, so we know which timed out.
				ctx, cancel := context.WithTimeout(goatcounter.CopyContextValues(r.Context()), 10*time.Second)
				defer cancel()

				l := zlog.Module("dashboard")
				err := w.GetData(ctx, args)
				if err != nil {
					l.FieldsRequest(r).Error(err)
					_, err = zhttp.UserError(err)
					w.SetErr(err)
				}
				l.Since(w.Name())
			}(w)
		}
		zsync.Wait(r.Context(), &wg)
	}()

	// Set shared params.
	shared := widgets.SharedData{Args: args, Site: site}
	tc := wid.Get("totalcount").(*widgets.TotalCount)
	shared.Total, shared.TotalUnique, shared.TotalUniqueUTC, shared.TotalEvents,
		shared.TotalEventsUnique = tc.Total, tc.TotalUnique, tc.TotalUniqueUTC,
		tc.TotalEvents, tc.TotalEventsUnique

	// Copy max and refs to pages; they're in separate "widgets" so they can run
	// in parallel.
	if p := wid.Get("pages"); p != nil {
		p.(*widgets.Pages).Max = wid.Get("max").(*widgets.Max).Max
		if showRefs != "" {
			p.(*widgets.Pages).Refs = wid.Get("refs").(*widgets.Refs).Refs
		}
	}

	// Render widget templates.
	func() {
		var wg sync.WaitGroup
		for _, w := range wid {
			wg.Add(1)
			go func(w widgets.Widget) {
				defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.Field("tpl widget", w).FieldsRequest(r) })
				defer wg.Done()

				tplName, tplData := w.RenderHTML(r.Context(), shared)
				if tplName == "" { // Some data doesn't have a template.
					return
				}
				tpl, err := ztpl.ExecuteString(tplName, tplData)
				if err != nil {
					zlog.Module("dashboard").FieldsRequest(r).Error(err)
					w.SetHTML(template.HTML("template rendering error: " + template.HTMLEscapeString(err.Error())))
					return
				}

				w.SetHTML(template.HTML(tpl))
			}(w)
		}
		zsync.Wait(r.Context(), &wg)
	}()

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
			"timerange": tplfunc.Daterange(site.Settings.Timezone.Loc(), start, end),
		})
	}

	return zhttp.Template(w, "dashboard.gohtml", struct {
		Globals
		CountDomain string
		SubSites    []string
		ShowRefs    string

		PeriodStart    time.Time
		PeriodEnd      time.Time
		PathFilter     []int64
		ForcedDaily    bool
		Widgets        widgets.List
		View           goatcounter.View
		TotalUnique    int
		TotalUniqueUTC int
	}{newGlobals(w, r), cd, subs, showRefs, start, end, args.PathFilter,
		forcedDaily, wid, view, shared.TotalUnique, shared.TotalUniqueUTC})
}

// Get a time range; the return value is always in UTC, and is the UTC day range
// corresponding to the given timezone.
//
// So, for example a week in +08:00 would be:
// 2020-12-20 16:00:00 - 2020-12-27 15:59:59
//
// Values for rng:
//
//   week, month, quarter, half-year, year
//      The start date is set to exactly this period ago. The end date is set to
//      the end of the current day.
//
//   week-cur, month-cur
//      The current week or month; both the start and return are modified.
//
//   Any digit
//      Last n days.
func timeRange(rng string, tz *time.Location, sundayStartsWeek bool) (time.Time, time.Time, error) {
	y, m, d := goatcounter.Now().In(tz).Date()
	end := time.Date(y, m, d, 23, 59, 59, 9, tz).Round(time.Second)

	var start time.Time
	switch rng {
	case "0", "day":
		start = time.Date(y, m, d, 0, 0, 0, 0, tz)
	case "week":
		start = time.Date(y, m, d-7, 0, 0, 0, 0, tz)
	case "month":
		start = time.Date(y, m-1, d, 0, 0, 0, 0, tz)
	case "quarter":
		start = time.Date(y, m-3, d, 0, 0, 0, 0, tz)
	case "half-year":
		start = time.Date(y, m-6, d, 0, 0, 0, 0, tz)
	case "year":
		start = time.Date(y, m-12, d, 0, 0, 0, 0, tz)

	case "week-cur":
		n := nnow.New(end)
		n.WeekStartDay = time.Monday
		if sundayStartsWeek {
			n.WeekStartDay = time.Sunday
		}

		start = n.BeginningOfWeek()
		end = n.EndOfWeek().Add(-1 * time.Second).Round(1 * time.Second)
	case "month-cur":
		n := nnow.New(end)
		n.WeekStartDay = time.Monday
		if sundayStartsWeek {
			n.WeekStartDay = time.Sunday
		}
		start = n.BeginningOfMonth()
		end = n.EndOfMonth().Add(-1 * time.Second).Round(1 * time.Second)

	default:
		days, err := strconv.Atoi(rng)
		if err != nil {
			zlog.Field("rng", rng).Error(err)
			return timeRange("week", tz, sundayStartsWeek)
		}
		start = time.Date(y, m, d-days, 0, 0, 0, 0, time.UTC)
	}

	return start.UTC(), end.UTC(), nil
}

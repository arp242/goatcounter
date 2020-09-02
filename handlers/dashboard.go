// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"html/template"
	"net/http"
	"sync"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/widgets"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
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

	hlPeriod := r.URL.Query().Get("hl-period")
	start, end, err := getPeriod(w, r, site)
	if err != nil {
		zhttp.FlashError(w, err.Error())
	}
	if start.IsZero() || end.IsZero() {
		y, m, d := goatcounter.Now().In(site.Settings.Timezone.Loc()).Date()
		now := time.Date(y, m, d, 0, 0, 0, 0, site.Settings.Timezone.Loc())
		start = now.Add(-7 * day).UTC()
		end = time.Date(y, m, d, 23, 59, 59, 9, now.Location()).UTC().Round(time.Second)
		hlPeriod = "week"
	}

	// Get path IDs to filter first, as they're used by the widgets.
	var (
		filter     = r.URL.Query().Get("filter")
		pathFilter = make(chan (struct {
			Paths []int64
			Err   error
		}))
	)
	go func() {
		defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.Field("filter", filter).FieldsRequest(r) })

		l := zlog.Module("dashboard")

		var (
			f   []int64
			err error
		)
		if filter != "" {
			f, err = goatcounter.PathFilter(r.Context(), filter, true)
		}
		pathFilter <- struct {
			Paths []int64
			Err   error
		}{f, err}
		l.Since("pathfilter")
	}()

	showRefs := r.URL.Query().Get("showrefs")
	asText := r.URL.Query().Get("as-text") != ""
	daily, forcedDaily := getDaily(r, start, end)

	subs, err := site.ListSubs(r.Context())
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

	args := widgets.Args{
		Start:       start,
		End:         end,
		Daily:       daily,
		ShowRefs:    showRefs,
		ForcedDaily: forcedDaily,
		AsText:      asText,
	}

	wantWidgets := goatcounter.GetUser(r.Context()).Widgets()
	if zstring.Contains(wantWidgets, "pages") {
		wantWidgets = append(wantWidgets, "max")
		if showRefs != "" {
			wantWidgets = append(wantWidgets, "refs")
		}
	}
	widgetList, err := widgets.NewList(wantWidgets)
	if err != nil {
		return err
	}

	f := <-pathFilter
	args.PathFilter, err = f.Paths, f.Err
	if err != nil {
		return err
	}

	// Load data.
	err = func() error {
		var (
			wg   sync.WaitGroup
			errs = errors.NewGroup(20)
		)
		for _, w := range widgetList {
			wg.Add(1)
			go func(w widgets.Widget) {
				defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.Field("data widget", w).FieldsRequest(r) })
				defer wg.Done()

				l := zlog.Module("dashboard")
				errs.Append(w.GetData(r.Context(), args))
				l.Since(w.Name())
			}(w)
		}

		zsync.Wait(r.Context(), &wg)
		return errs.ErrorOrNil()
	}()
	if err != nil {
		return err
	}

	shared := widgets.SharedData{Args: args, Site: site}
	shared.Total, shared.TotalUnique, shared.AllTotalUniqueUTC, shared.Max = widgetList.Totals()
	if showRefs != "" {
		shared.Refs = widgetList.Refs()
	}

	// Render templates.
	err = func() error {
		var (
			wg   sync.WaitGroup
			errs = errors.NewGroup(20)
		)
		for _, w := range widgetList {
			wg.Add(1)
			go func(w widgets.Widget) {
				defer zlog.Recover(func(l zlog.Log) zlog.Log { return l.Field("tpl widget", w).FieldsRequest(r) })
				defer wg.Done()

				tplName, tplData := w.TemplateData(r.Context(), shared)
				if tplName == "" { // Some data doesn't have a template.
					return
				}
				tpl, err := ztpl.ExecuteString(tplName, tplData)
				if errs.Append(errors.Wrap(err, w.Name())) {
					return
				}
				w.SetHTML(template.HTML(tpl))
			}(w)
		}

		zsync.Wait(r.Context(), &wg)
		return errs.ErrorOrNil()
	}()
	if err != nil {
		return err
	}

	return zhttp.Template(w, "dashboard.gohtml", struct {
		Globals
		CountDomain    string
		SubSites       []string
		ShowRefs       string
		SelectedPeriod string
		PeriodStart    time.Time
		PeriodEnd      time.Time
		Filter         string
		PathFilter     []int64
		Daily          bool
		ForcedDaily    bool
		AsText         bool
		Widgets        widgets.List
	}{newGlobals(w, r),
		cd, subs, showRefs, hlPeriod, start, end, filter, args.PathFilter,
		daily, forcedDaily, asText, widgetList})
}

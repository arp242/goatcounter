// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/bgrun"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/metrics"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/auth"
	"zgo.at/zhttp/mware"
	"zgo.at/zprof"
	"zgo.at/zstd/znet"
	"zgo.at/zstd/ztime"
	"zgo.at/zvalidate"
)

type bosmang struct{}

func (h bosmang) mount(r chi.Router, db zdb.DB) {
	a := r.With(mware.RequestLog(nil), requireAccess(goatcounter.AccessSuperuser))

	r.Get("/bosmang", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/settings/server")
	}))

	a.Get("/bosmang/cache", zhttp.Wrap(h.cache))
	a.Get("/bosmang/bgrun", zhttp.Wrap(h.bgrun))
	a.Post("/bosmang/bgrun/{task}", zhttp.Wrap(h.runTask))
	a.Get("/bosmang/metrics", zhttp.Wrap(h.metrics))
	a.Handle("/bosmang/profile*", zprof.NewHandler(zprof.Prefix("/bosmang/profile")))

	a.Get("/bosmang/sites", zhttp.Wrap(h.sites))
	a.Get("/bosmang/sites/{id}", zhttp.Wrap(h.site))
	a.Post("/bosmang/sites/{id}/update-billing", zhttp.Wrap(h.updateBilling))
	a.Post("/bosmang/sites/login/{id}", zhttp.Wrap(h.login))
}

func (h bosmang) cache(w http.ResponseWriter, r *http.Request) error {
	cache := goatcounter.ListCache(r.Context())
	return zhttp.Template(w, "bosmang_cache.gohtml", struct {
		Globals
		Cache map[string]struct {
			Size  int64
			Items map[string]string
		}
	}{newGlobals(w, r), cache})
}

func (h bosmang) bgrun(w http.ResponseWriter, r *http.Request) error {
	hist := bgrun.History()

	metrics := make(map[string]ztime.Durations)
	for _, h := range hist {
		x, ok := metrics[h.Name]
		if !ok {
			x = ztime.NewDurations(0)
			x.Grow(32)
		}
		x.Append(h.Finished.Sub(h.Started))
		metrics[h.Name] = x
	}

	return zhttp.Template(w, "bosmang_bgrun.gohtml", struct {
		Globals
		Tasks   []cron.Task
		Jobs    []bgrun.Job
		History []bgrun.Job
		Metrics map[string]ztime.Durations
	}{newGlobals(w, r), cron.Tasks, bgrun.List(), hist, metrics})
}

func (h bosmang) runTask(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
	taskID := v.Integer("task", chi.URLParam(r, "task"))
	v.Range("task", taskID, 0, int64(len(cron.Tasks)-1))
	if v.HasErrors() {
		return v
	}

	t := cron.Tasks[taskID]
	id := t.ID()
	bgrun.Run("manual:"+id, func() {
		t.Fun(r.Context())
	})

	zhttp.Flash(w, "Task %q started", id)
	return zhttp.SeeOther(w, "/bosmang/bgrun")
}

func (h bosmang) metrics(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "bosmang_metrics.gohtml", struct {
		Globals
		Metrics metrics.Metrics
	}{newGlobals(w, r), metrics.List()})
}

func (h bosmang) sites(w http.ResponseWriter, r *http.Request) error {
	var (
		wg         sync.WaitGroup
		signups    []goatcounter.HitListStat
		maxSignups int
		bgErr      = errors.NewGroup(20)
	)
	go func() {
		var sites goatcounter.Sites
		err := sites.UnscopedList(r.Context())
		if bgErr.Append(err) {
			return
		}
		grouped := make(map[string]int) // day → count
		cutoff := time.Now().Add(-365 * 24 * time.Hour)
		for _, s := range sites {
			if s.Parent != nil {
				continue
			}
			if s.CreatedAt.Before(cutoff) {
				continue
			}
			grouped[s.CreatedAt.Format("2006-01-02")]++
		}

		for k, v := range grouped {
			if v > maxSignups {
				maxSignups = v
			}
			signups = append(signups, goatcounter.HitListStat{
				Day:          k,
				Hourly:       []int{v},
				HourlyUnique: []int{v},
			})
		}
		sort.Slice(signups, func(i, j int) bool { return signups[i].Day < signups[j].Day })
	}()

	var a goatcounter.BosmangStats
	err := a.List(r.Context())
	if err != nil {
		return err
	}

	wg.Wait()
	if bgErr.Len() > 0 {
		return bgErr
	}

	return zhttp.Template(w, "bosmang_sites.gohtml", struct {
		Globals
		Stats      goatcounter.BosmangStats
		Signups    []goatcounter.HitListStat
		MaxSignups int
	}{newGlobals(w, r), a, signups, maxSignups})
}

func (h bosmang) site(w http.ResponseWriter, r *http.Request) error {
	var a goatcounter.BosmangSiteStat
	err := a.Find(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if zdb.ErrNoRows(err) {
			return guru.New(404, "no such site")
		}
		return err
	}

	return zhttp.Template(w, "bosmang_site.gohtml", struct {
		Globals
		Stat goatcounter.BosmangSiteStat
	}{newGlobals(w, r), a})
}

func (h bosmang) updateBilling(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))

	var args struct {
		Stripe       string `json:"stripe"`
		Amount       string `json:"amount"`
		Plan         string `json:"plan"`
		PlanPending  string `json:"plan_pending"`
		PlanCancelAt string `json:"plan_cancel_at"`
		Notes        string `json:"notes"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/bosmang/%d", id))
	}

	var site goatcounter.Site
	err = site.ByID(r.Context(), id)
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/bosmang/%d", id))
	}

	site.Stripe, site.BillingAmount, site.PlanPending, site.PlanCancelAt = nil, nil, nil, nil

	site.Plan = args.Plan
	if args.Stripe != "" {
		site.Stripe = &args.Stripe
	}
	if args.Amount != "" {
		site.BillingAmount = &args.Amount
	}
	if args.PlanPending != "" {
		site.PlanPending = &args.PlanPending
	}
	if args.PlanCancelAt != "" {
		t, err := time.Parse("2006-01-02 15:04:05", args.PlanCancelAt)
		if err != nil {
			return err
		}
		site.PlanCancelAt = &t
	}

	ctx := goatcounter.WithSite(goatcounter.CopyContextValues(r.Context()), &site)
	err = zdb.TX(ctx, func(ctx context.Context) error {
		err := site.UpdateStripe(ctx)
		if err != nil {
			return err
		}
		return zdb.Exec(ctx, `update sites set notes=? where site_id=?`, args.Notes, site.ID)
	})
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/bosmang/%d", id))
	}

	return zhttp.SeeOther(w, fmt.Sprintf("/bosmang/%d", id))
}

func (h bosmang) login(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))
	if v.HasErrors() {
		return v
	}

	var site goatcounter.Site
	err := site.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	var users goatcounter.Users
	err = users.List(r.Context(), site.ID)
	if err != nil {
		return err
	}
	user := users[0]

	if !site.Settings.AllowBosmang {
		return guru.New(403, "AllowBosmang not enabled")
	}

	domain := cookieDomain(&site, r)
	auth.SetCookie(w, *user.LoginToken, domain)
	http.SetCookie(w, &http.Cookie{
		Domain:   znet.RemovePort(domain),
		Name:     "is_bosmang",
		Value:    "1",
		Path:     "/",
		Expires:  time.Now().Add(8 * time.Hour),
		HttpOnly: true,
		Secure:   zhttp.CookieSecure,
		SameSite: zhttp.CookieSameSite,
	})

	return zhttp.SeeOther(w, site.URL(r.Context()))
}

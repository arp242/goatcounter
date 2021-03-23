// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/auth"
	"zgo.at/zhttp/mware"
	"zgo.at/zstd/znet"
	"zgo.at/zvalidate"
)

type bosmang struct{}

func (h bosmang) mount(r chi.Router, db zdb.DB) {
	a := r.With(mware.RequestLog(nil), bosmangOnly)
	a.Get("/bosmang", zhttp.Wrap(h.index))
	a.Get("/bosmang/{id}", zhttp.Wrap(h.site))
	a.Post("/bosmang/{id}/update-billing", zhttp.Wrap(h.updateBilling))
	a.Post("/bosmang/login/{id}", zhttp.Wrap(h.login))

	a.Get("/debug/*", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/debug/pprof") {
			pprof.Index(w, r)
			return
		}
		zhttp.SeeOther(w, fmt.Sprintf("/debug/pprof/%s?%s",
			r.URL.Path[7:], r.URL.Query().Encode()))
	})
	a.Get("/debug/pprof/cmdline", pprof.Cmdline)
	a.Get("/debug/pprof/profile", pprof.Profile)
	a.Get("/debug/pprof/symbol", pprof.Symbol)
	a.Get("/debug/pprof/trace", pprof.Trace)
}

func (h bosmang) index(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

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

	return zhttp.Template(w, "bosmang.gohtml", struct {
		Globals
		Stats      goatcounter.BosmangStats
		Signups    []goatcounter.HitListStat
		MaxSignups int
	}{newGlobals(w, r), a, signups, maxSignups})
}

func (h bosmang) site(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

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
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))

	var args struct {
		Stripe       string `json:"stripe"`
		Amount       string `json:"amount"`
		Plan         string `json:"plan"`
		PlanPending  string `json:"plan_pending"`
		PlanCancelAt string `json:"plan_cancel_at"`
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
	err = site.UpdateStripe(ctx)
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/bosmang/%d", id))
	}

	return zhttp.SeeOther(w, fmt.Sprintf("/bosmang/%d", id))
}

func (h bosmang) login(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

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

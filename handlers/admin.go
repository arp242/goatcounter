// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/http/pprof"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"zgo.at/goatcounter"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

type admin struct{}

func (h admin) mount(r chi.Router) {
	//a := r.With(zhttp.Log(true, ""), keyAuth, adminOnly)
	a := r.With(zhttp.Log(true, ""), adminOnly)

	a.Get("/admin", zhttp.Wrap(h.index))
	a.Get("/admin/sql", zhttp.Wrap(h.sql))
	a.Get("/admin/botlog", zhttp.Wrap(h.botlog))
	a.Get("/admin/{id}", zhttp.Wrap(h.site))
	a.Post("/admin/{id}/gh-sponsor", zhttp.Wrap(h.ghSponsor))

	//aa.Get("/debug/pprof/*", pprof.Index)
	a.Get("/debug/*", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/debug/pprof") {
			pprof.Index(w, r)
		}
		zhttp.SeeOther(w, fmt.Sprintf("/debug/pprof/%s?%s",
			r.URL.Path[7:], r.URL.Query().Encode()))
	})
	a.Get("/debug/pprof/cmdline", pprof.Cmdline)
	a.Get("/debug/pprof/profile", pprof.Profile)
	a.Get("/debug/pprof/symbol", pprof.Symbol)
	a.Get("/debug/pprof/trace", pprof.Trace)
}

func (h admin) index(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	l := zlog.Module("admin")

	var a goatcounter.AdminStats
	err := a.List(r.Context())
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
	cutoff := time.Now().Add(-120 * 24 * time.Hour)
	for _, s := range sites {
		if s.Parent != nil {
			continue
		}
		if s.CreatedAt.Before(cutoff) {
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
	sort.Slice(signups, func(i, j int) bool { return signups[i].Day < signups[j].Day })

	l = l.Since("signups")
	var (
		totalUSD int
		totalEUR int
	)
	for _, s := range a {
		if s.BillingAmount == nil {
			continue
		}
		b := *s.BillingAmount
		n, _ := strconv.ParseInt(b[4:], 10, 32)
		fmt.Println(b, ">", n)

		if strings.HasPrefix(*s.BillingAmount, "EUR ") {
			totalEUR += int(n)
		} else {
			totalUSD += int(n)
		}
	}
	totalEarnings := totalEUR + int(math.Round((float64(totalUSD)+24)*0.9)) // $24 from Patreon

	l.FieldsSince().Debug("admin")
	return zhttp.Template(w, "admin.gohtml", struct {
		Globals
		Stats         goatcounter.AdminStats
		Signups       []goatcounter.Stat
		MaxSignups    int
		TotalUSD      int
		TotalEUR      int
		TotalEarnings int
	}{newGlobals(w, r), a, signups, maxSignups, totalUSD, totalEUR, totalEarnings})
}

func (h admin) sql(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var load string
	uptime, err := exec.Command("uptime").CombinedOutput()
	if err == nil {
		load = strings.TrimSpace(strings.Join(strings.Split(string(uptime), ",")[2:], ", "))
	}
	free, err := exec.Command("free", "-m").CombinedOutput()
	if err != nil {
		free = nil
	}
	// Ignore exit/stderr because:
	// df: /sys/kernel/debug/tracing: Permission denied
	df, _ := exec.Command("df", "-hT").Output()

	filter := r.URL.Query().Get("filter")
	order := r.URL.Query().Get("order")
	asc := r.URL.Query().Get("asc") != ""

	var stats goatcounter.AdminPgStatStatements
	err = stats.List(r.Context(), order, asc, filter)
	if err != nil {
		return err
	}

	var act goatcounter.AdminPgStatActivity
	err = act.List(r.Context())
	if err != nil {
		return err
	}

	var tbls goatcounter.AdminPgStatTables
	err = tbls.List(r.Context())
	if err != nil {
		return err
	}

	var idx goatcounter.AdminPgStatIndexes
	err = idx.List(r.Context())
	if err != nil {
		return err
	}

	var prog goatcounter.AdminPgStatProgress
	err = prog.List(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "admin_sql.gohtml", struct {
		Globals
		Filter   string
		Order    string
		Load     string
		Free     string
		Df       string
		Stats    goatcounter.AdminPgStatStatements
		Activity goatcounter.AdminPgStatActivity
		Tables   goatcounter.AdminPgStatTables
		Indexes  goatcounter.AdminPgStatIndexes
		Progress goatcounter.AdminPgStatProgress
	}{newGlobals(w, r), filter, order, load, string(free), string(df), stats, act, tbls,
		idx, prog})
}

func (h admin) botlog(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var ips goatcounter.AdminBotlogIPs
	err := ips.List(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "admin_botlog.gohtml", struct {
		Globals
		BotlogIP goatcounter.AdminBotlogIPs
	}{newGlobals(w, r), ips})
}

func (h admin) site(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
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

	return zhttp.Template(w, "admin_site.gohtml", struct {
		Globals
		Stat goatcounter.AdminSiteStat
	}{newGlobals(w, r), a})
}

func (h admin) ghSponsor(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	v := zvalidate.New()
	id := v.Integer("id", chi.URLParam(r, "id"))

	var args struct {
		Stripe string `json:"stripe"`
		Amount string `json:"amount"`
		Plan   string `json:"plan"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/admin/%d", id))
	}

	v.Required("plan", args.Plan)
	v.Include("plan", args.Plan, goatcounter.Plans)
	if v.HasErrors() {
		zhttp.FlashError(w, v.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/admin/%d", id))
	}

	var site goatcounter.Site
	err = site.ByID(r.Context(), id)
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/admin/%d", id))
	}

	c := "EUR"
	if strings.HasPrefix(args.Stripe, "cus_github") {
		c = "USD"
	}
	if args.Amount != "" && !strings.HasPrefix(args.Amount, c) {
		args.Amount = c + " " + args.Amount
	}

	ctx := goatcounter.WithSite(goatcounter.NewContext(r.Context()), &site)
	err = site.UpdateStripe(ctx, args.Stripe, args.Plan, args.Amount)
	if err != nil {
		zhttp.FlashError(w, err.Error())
		return zhttp.SeeOther(w, fmt.Sprintf("/admin/%d", id))
	}

	return zhttp.SeeOther(w, fmt.Sprintf("/admin/%d", id))
}

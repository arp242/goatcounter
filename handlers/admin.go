// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os/exec"
	"sort"
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

	a.Get("/admin", zhttp.Wrap(h.admin))
	a.Get("/admin/sql", zhttp.Wrap(h.adminSQL))
	a.Get("/admin/botlog", zhttp.Wrap(h.adminBotlog))
	a.Get("/admin/{id}", zhttp.Wrap(h.adminSite))

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

func (h admin) admin(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
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

	l.FieldsSince().Debug("admin")
	return zhttp.Template(w, "backend_admin.gohtml", struct {
		Globals
		Stats      goatcounter.AdminStats
		Signups    []goatcounter.Stat
		MaxSignups int
	}{newGlobals(w, r), a, signups, maxSignups})
}

func (h admin) adminSQL(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
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
	df, err := exec.Command("df", "-hT").Output()
	if err != nil {
		df = nil
	}

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

	return zhttp.Template(w, "backend_admin_sql.gohtml", struct {
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

func (h admin) adminBotlog(w http.ResponseWriter, r *http.Request) error {
	if goatcounter.MustGetSite(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var ips goatcounter.AdminBotlogIPs
	err := ips.List(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "backend_admin_botlog.gohtml", struct {
		Globals
		BotlogIP goatcounter.AdminBotlogIPs
	}{newGlobals(w, r), ips})
}

func (h admin) adminSite(w http.ResponseWriter, r *http.Request) error {
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

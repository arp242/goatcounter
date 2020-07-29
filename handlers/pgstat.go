// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-chi/chi"
	"zgo.at/goatcounter/pgstat"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

func (h admin) explain(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var args struct {
		Query string `json:"query"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	//`explain (analyze, costs, verbose, buffers, format json) `+args.Query)

	var e []string
	err = zdb.MustGet(r.Context()).SelectContext(r.Context(), &e,
		`explain analyze `+args.Query)
	if err != nil {
		return err
	}

	plan := strings.Join(e, "\n")
	plan = regexp.MustCompile(`cost=[0-9.]+`).ReplaceAllString(plan, `<span class="cost">$0</span>`)

	return zhttp.String(w, plan)
}

func (h admin) pgstatTable(w http.ResponseWriter, r *http.Request) error {
	if Site(r.Context()).ID != 1 {
		return guru.New(403, "yeah nah")
	}

	var stats pgstat.TableStat
	err := stats.List(r.Context(), chi.URLParam(r, "table"))
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(`<table class="sort"><thead><tr>
		<th>Column</th>
		<th class="n">NullFrac</th>
		<th class="n">AvgWidth</th>
		<th class="n">NDistinct</th>
		<th class="n">Correlation</th>
	</tr></thead><tobdy>`)
	for _, s := range stats {
		b.WriteString(fmt.Sprintf(`<tr>
			<td>%s</td>
			<td class="n">%.3f</td>
			<td class="n">%d</td>
			<td class="n">%f</td>
			<td class="n">%f</td>
		</tr>`,
			template.HTMLEscapeString(s.AttName),
			s.NullFrac, s.AvgWidth, s.NDistinct, s.Correlation))
	}
	b.WriteString(`</tbody></table>`)

	return zhttp.String(w, b.String())
}

func (h admin) pgstat(w http.ResponseWriter, r *http.Request) error {
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

	var stats pgstat.Statements
	err = stats.List(r.Context(), order, asc, filter)
	if err != nil {
		return err
	}

	var act pgstat.Activity
	err = act.List(r.Context())
	if err != nil {
		return err
	}

	var tbls pgstat.Tables
	err = tbls.List(r.Context())
	if err != nil {
		return err
	}

	var idx pgstat.Indexes
	err = idx.List(r.Context())
	if err != nil {
		return err
	}

	var prog pgstat.Progress
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
		Stats    pgstat.Statements
		Activity pgstat.Activity
		Tables   pgstat.Tables
		Indexes  pgstat.Indexes
		Progress pgstat.Progress
	}{newGlobals(w, r), filter, order, load, string(free), string(df), stats, act, tbls,
		idx, prog})
}

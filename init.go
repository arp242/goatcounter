// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

//go:generate go run gen.go

package goatcounter

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

func init() {
	var (
		ss = time.Date(2019, 9, 16, 0, 0, 0, 0, time.UTC)
		sl = time.Date(2019, 11, 7, 0, 0, 0, 0, time.UTC)
	)
	zhttp.FuncMap["beforeSize"] = func(createdAt time.Time) bool { return createdAt.Before(ss) }
	zhttp.FuncMap["beforeLoc"] = func(createdAt time.Time) bool { return createdAt.Before(sl) }
	zhttp.FuncMap["error_code"] = func(err error) string { return zhttp.ErrorCode(err) }

	zhttp.FuncMap["parent_site"] = func(ctx context.Context, id *int64) string {
		var s Site
		err := s.ByID(ctx, *id)
		if err != nil {
			zlog.Error(err)
			return ""
		}
		return s.URL()
	}

	zhttp.FuncMap["validate"] = func(k string, v map[string][]string) template.HTML {
		if v == nil {
			return template.HTML("")
		}
		e, ok := v[k]
		if !ok {
			return template.HTML("")
		}
		return template.HTML(fmt.Sprintf(`<span class="err">Error: %s</span>`,
			template.HTMLEscapeString(strings.Join(e, ", "))))
	}

	// Implemented as function for performance.
	zhttp.FuncMap["bar_chart"] = func(stats []HitStat, max int) template.HTML {
		var b strings.Builder
		now := time.Now().UTC()
		today := now.Format("2006-01-02")
		hour := now.Hour()
		for _, stat := range stats {
			for shour, s := range stat.Days {
				// Don't show stuff in the future.
				if stat.Day == today && shour > hour {
					break
				}
				h := math.Round(float64(s) / float64(max) / 0.01)

				// Double div so that the title is on the entire column, instead
				// of just the coloured area.
				// No need to add the inner one if there's no data – saves quite
				// a bit in the total filesize.
				inner := ""
				if h > 0 {
					inner = fmt.Sprintf(`<div style="height: %.0f%%;"></div>`, h)
				}
				b.WriteString(fmt.Sprintf(`<div title="%s %[2]d:00 – %[2]d:59, %s views">%s</div>`,
					stat.Day, shour, zhttp.Tnformat(s), inner))
			}
		}

		return template.HTML(b.String())
	}

	zhttp.FuncMap["hbar_chart"] = func(stats BrowserStats, total, parentTotal int, cutoff float32, link bool) template.HTML {
		tag := "p"
		if link {
			tag = "a"
		}

		totalPerc := float32(0.0)
		var b strings.Builder
		for _, s := range stats {
			perc := float32(s.Count) / float32(total) * 100
			if perc < cutoff { // Group as "Other" later.
				break
			}
			totalPerc += perc

			browser := s.Browser
			if browser == "" {
				browser = "(unknown)"
			}

			title := fmt.Sprintf("%s: %.1f%% – ", template.HTMLEscapeString(browser), perc)
			if parentTotal > 0 {
				title += fmt.Sprintf("%.1f%% of total, %s hits",
					float32(s.Count)/float32(parentTotal)*100, zhttp.Tnformat(s.Count))
			} else {
				title += fmt.Sprintf("%s hits in total", zhttp.Tnformat(s.Count))
			}

			b.WriteString(fmt.Sprintf(
				`<%[4]s href="#_" title="%[1]s"><small>%[2]s</small> <span style="width: %[3]f%%">%.1[3]f%%</span></%[4]s>`,
				title, template.HTMLEscapeString(browser), perc, tag))
		}

		// Add "(other)" part.
		if totalPerc < 100 {
			b.WriteString(fmt.Sprintf(
				`<%[2]s href="#_" title="(other): %.1[1]f%%" class="other"><small>(other)</small> <span style="width: %[1]f%%">%.1[1]f%%</span></%[2]s>`,
				100-totalPerc, tag))
		}

		return template.HTML(b.String())
	}
}

// State column values.
const (
	StateActive  = "a"
	StateRequest = "r"
	StateDeleted = "d"
)

var States = []string{StateActive, StateRequest, StateDeleted}

// GetSite gets the current site.
func GetSite(ctx context.Context) *Site {
	s, _ := ctx.Value(ctxkey.Site).(*Site)
	return s
}

// MustGetSite behaves as GetSite(), panicking if this fails.
func MustGetSite(ctx context.Context) *Site {
	s, ok := ctx.Value(ctxkey.Site).(*Site)
	if !ok {
		panic("MustGetSite: no site on context")
	}
	return s
}

// GetUser gets the currently logged in user.
func GetUser(ctx context.Context) *User {
	u, _ := ctx.Value(ctxkey.User).(*User)
	return u
}

// NewContext creates a new context with the all the request values set.
func NewContext(ctx context.Context) context.Context {
	n := zdb.With(context.Background(), zdb.MustGet(ctx))
	n = context.WithValue(n, ctxkey.User, GetUser(ctx))
	n = context.WithValue(n, ctxkey.Site, GetSite(ctx))
	return n
}

func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

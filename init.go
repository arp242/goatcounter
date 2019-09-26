// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

//go:generate go run gen.go

package goatcounter

import (
	"context"
	"crypto/md5"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"zgo.at/goatcounter/cfg"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

func init() {
	zhttp.FuncMap["parent_site"] = func(ctx context.Context, id *int64) template.HTML {
		var s Site
		err := s.ByID(ctx, *id)
		if err != nil {
			zlog.Error(err)
			return template.HTML("")
		}
		return template.HTML(fmt.Sprintf(`<a href="//%s.%s">%[1]s</a>`,
			s.Code, cfg.Domain))
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
			for _, s := range stat.Days {
				// Don't show stuff in the future.
				if stat.Day == today && s[0] > hour {
					break
				}
				h := math.Round(float64(s[1]) / float64(max) / 0.01)

				// Double div so that the title is on the entire column, instead
				// of just the coloured area.
				// No need to add the inner one if there's no data – saves quite
				// a bit in the total filesize.
				inner := ""
				if h > 0 {
					inner = fmt.Sprintf(`<div style="height: %.0f%%;"></div>`, h)
				}
				b.WriteString(fmt.Sprintf(`<div title="%s %[2]d:00 – %[2]d:59, %s views">%s</div>`,
					stat.Day, s[0], zhttp.Tnformat(s[1]), inner))
			}
		}

		return template.HTML(b.String())
	}

	zhttp.FuncMap["hbar_chart"] = func(stats BrowserStats, total, parentTotal int, cutoff float32) template.HTML {
		totalPerc := float32(0.0)
		var b strings.Builder
		for _, s := range stats {
			perc := float32(s.Count) / float32(total) * 100
			if perc < cutoff {
				// Less than cutoff percentage: group as "Other" later.
				break
			}
			totalPerc += perc

			browser := s.Browser
			if browser == "" {
				browser = "(unknown)"
			}

			bg, fg := colorHash(browser)
			text := fmt.Sprintf("%s: %.1f%% – ", template.HTMLEscapeString(browser), perc)
			if parentTotal > 0 {
				text += fmt.Sprintf("%.1f%% of total, %s hits",
					float32(s.Count)/float32(parentTotal)*100, zhttp.Tnformat(s.Count))
			} else {
				text += fmt.Sprintf("%s hits in total", zhttp.Tnformat(s.Count))
			}

			b.WriteString(fmt.Sprintf(
				`<a href="#_" title="%[1]s" style="width: %[2]f%%; background-color: %[3]s; color: %[4]s" data-browser="%[5]s">%[1]s</a>`,
				text, perc, bg, fg, browser))
		}

		// Add "(other)" part.
		if totalPerc < 100 {
			b.WriteString(fmt.Sprintf(
				`<a href="#_" title="Other: %.1[1]f%%" style="width: %[1]f%%">Other: %.1[1]f%%</a>`, 100-totalPerc))
		}

		return template.HTML(b.String())
	}
}

func colorHash(s string) (string, string) {
	hash := md5.New()
	hash.Write([]byte(s))
	color := string(hash.Sum(nil))
	fg := "#000"
	if .299*float32(color[0])+.587*float32(color[1])+.114*float32(color[2]) < 150 {
		fg = "#fff"
	}
	return fmt.Sprintf("#%.2x%.2x%.2x", color[0], color[1], color[2]), fg
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

func dayStart(t time.Time) string { return t.Format("2006-01-02") + " 00:00:00" }
func dayEnd(t time.Time) string   { return t.Format("2006-01-02") + " 23:59:59" }

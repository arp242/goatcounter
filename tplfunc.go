// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

func init() {
	zhttp.FuncMap["has_flag"] = HasFlag
	zhttp.FuncMap["validate"] = zvalidate.TemplateError
	zhttp.FuncMap["has_errors"] = zvalidate.TemplateHasErrors
	zhttp.FuncMap["error_code"] = func(err error) string { return zhttp.ErrorCode(err) }
	zhttp.FuncMap["nformat2"] = func(n int, s Site) string {
		return zhttp.FuncMap["nformat"].(func(int, rune) string)(n, s.Settings.NumberFormat)
	}
	zhttp.FuncMap["parent_site"] = func(ctx context.Context, id *int64) string {
		var s Site
		err := s.ByID(ctx, *id)
		if err != nil {
			zlog.Error(err)
			return ""
		}
		return s.URL()
	}

	var (
		ss = time.Date(2019, 9, 16, 0, 0, 0, 0, time.UTC)
		sl = time.Date(2019, 11, 7, 0, 0, 0, 0, time.UTC)
	)
	zhttp.FuncMap["before_size"] = func(createdAt time.Time) bool { return createdAt.Before(ss) }
	zhttp.FuncMap["before_loc"] = func(createdAt time.Time) bool { return createdAt.Before(sl) }

	// Implemented as function for performance.
	zhttp.FuncMap["bar_chart"] = BarChart
	zhttp.FuncMap["horizontal_chart"] = HorizontalChart

	// Override default to display in site TZ.
	zhttp.FuncMap["tformat"] = func(s *Site, t time.Time, fmt string) string {
		if fmt == "" {
			fmt = "2006-01-02"
		}
		return t.In(s.Settings.Timezone.Loc()).Format(fmt)
	}
}

func BarChart(ctx context.Context, stats []Stat, max int, daily bool) template.HTML {
	site := MustGetSite(ctx)

	now := Now().In(site.Settings.Timezone.Loc())
	_, offset := now.Zone()
	if offset%3600 != 0 {
		// Round to next hour for TZ offset of 9.5 hours, instead of down.
		offset += 1900
	}
	offset /= 3600

	// Daily view.
	// TODO: apply TZ offsets, e.g. if UTC+8 and local time is after 16:00, then
	// add hours from next day.
	if daily {
		var b strings.Builder
		for _, stat := range stats {
			inner := ""
			h := math.Round(float64(stat.Total) / float64(max) / 0.01)
			if h > 0 {
				inner = fmt.Sprintf(`<div style="height: %.0f%%;"></div>`, h)
			}
			b.WriteString(fmt.Sprintf(`<div title="%s, %s views">%s</div>`,
				stat.Day, zhttp.Tnformat(stat.Total, site.Settings.NumberFormat), inner))
		}

		return template.HTML(b.String())
	}

	var b strings.Builder
	today := now.Format("2006-01-02")
	hour := now.Hour()
	for _, stat := range stats {
		for shour, s := range stat.Days {
			// Don't show stuff in the future.
			if stat.Day == today && shour > hour {
				break
			}

			// Double div so that the title is on the entire column, instead of
			// just the coloured area. No need to add the inner one if there's
			// no data – saves quite a bit in the total filesize.
			inner := ""
			h := math.Round(float64(s) / float64(max) / 0.01)
			if h > 0 {
				inner = fmt.Sprintf(`<div style="height: %.0f%%"></div>`, h)
			}
			b.WriteString(fmt.Sprintf(`<div title="%s %[2]d:00 – %[2]d:59, %s views">%s</div>`,
				stat.Day, shour, zhttp.Tnformat(s, site.Settings.NumberFormat), inner))
		}
	}

	return template.HTML(b.String())
}

// The database stores everything in UTC, so we need to apply
// the offset.
//
// Let's say we have two days with an offset of UTC+2, this means we
// need to transform this:
//
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//    2019-12-06 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//    2019-12-07 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// To:
//
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//    2019-12-06 → [1,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//    2019-12-07 → [1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// And skip the first 2 hours of the first day.
//
// Or, for UTC-2:
//
//    2019-12-04 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//    2019-12-06 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//
// And skip the last 2 hours of the last day.
//
// Offsets that are not whole hours (e.g. 6:30) are treated like 7:00. I don't
// know how to do that otherwise.
func applyOffset(offset int, stats []Stat) []Stat {
	switch {
	case offset > 0:
		popped := make([]int, offset)
		for i := range stats {
			stats[i].Days = append(popped, stats[i].Days...)
			o := len(stats[i].Days) - offset
			popped = stats[i].Days[o:]
			stats[i].Days = stats[i].Days[:o]
		}
		stats = stats[1:] // Overselect a day to get the stats for it, remove it.

	case offset < 0:
		offset = -offset
		popped := make([]int, offset)
		for i := len(stats) - 1; i >= 0; i-- {
			stats[i].Days = append(stats[i].Days, popped...)
			popped = stats[i].Days[:offset]
			stats[i].Days = stats[i].Days[offset:]
		}
		stats = stats[:len(stats)-1] // Overselect a day to get the stats for it, remove it.
	}

	return stats
}

func HorizontalChart(ctx context.Context, stats Stats, total, parentTotal int, cutoff float32, link, other bool) template.HTML {
	tag := "p"
	if link {
		tag = "a"
	}

	totalPerc := float32(0.0)
	var b strings.Builder
	for _, s := range stats {
		perc := float32(s.Count) / float32(total) * 100
		totalPerc += perc
		if parentTotal > 0 {
			perc = float32(s.Count) / float32(parentTotal) * 100
		}
		if perc < cutoff { // Group as "Other" later.
			break
		}

		browser := s.Name
		if browser == "" {
			browser = "(unknown)"
		}

		title := fmt.Sprintf("%s: %.1f%% – %s hits in total",
			template.HTMLEscapeString(browser), perc,
			zhttp.Tnformat(s.Count, MustGetSite(ctx).Settings.NumberFormat))
		b.WriteString(fmt.Sprintf(
			`<%[4]s href="#_" title="%[1]s"><small>%[2]s</small> <span style="width: %[3]f%%">%.1[3]f%%</span></%[4]s>`,
			title, template.HTMLEscapeString(browser), perc, tag))
	}

	// Add "(other)" part.
	if other && totalPerc < 100 {
		b.WriteString(fmt.Sprintf(
			`<%[2]s href="#_" title="(other): %.1[1]f%%" class="other"><small>(other)</small> <span style="width: %[1]f%%">%.1[1]f%%</span></%[2]s>`,
			100-totalPerc, tag))
	}

	return template.HTML(b.String())
}

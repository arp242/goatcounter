// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"crypto/sha1"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"

	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

func init() {
	zhttp.FuncMap["validate"] = zvalidate.TemplateError
	zhttp.FuncMap["has_errors"] = zvalidate.TemplateHasErrors
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
	zhttp.FuncMap["hash"] = func(s string) string {
		h := sha1.New()
		h.Write([]byte(s))
		return fmt.Sprintf("%x", h.Sum(nil))
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

	// Format a duration
	zhttp.FuncMap["dformat"] = func(d time.Duration) string {
		return d.String() // TODO: better, also move to zhttp
	}

	// Override defaults to take site settings in to account.
	zhttp.FuncMap["tformat"] = func(s *Site, t time.Time, fmt string) string {
		if fmt == "" {
			fmt = "2006-01-02"
		}
		return t.In(s.Settings.Timezone.Loc()).Format(fmt)
	}
	zhttp.FuncMap["nformat"] = func(n int, s Site) string {
		return zhttp.Tnformat(n, s.Settings.NumberFormat)
	}

	zhttp.FuncMap["nformat64"] = func(n int64) string {
		s := strconv.FormatInt(n, 10)
		if len(s) < 4 {
			return s
		}

		b := []byte(s)
		for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
			b[i], b[j] = b[j], b[i]
		}

		var out []rune
		for i := range b {
			if i > 0 && i%3 == 0 && ',' > 1 {
				out = append(out, ',')
			}
			out = append(out, rune(b[i]))
		}

		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return string(out)
	}
}

func BarChart(ctx context.Context, stats []Stat, max int, daily bool) template.HTML {
	site := MustGetSite(ctx)

	now := Now().In(site.Settings.Timezone.Loc())

	var b strings.Builder
	future := false
	today := now.Format("2006-01-02")

	switch daily {
	// Daily view.
	case true:
		for _, stat := range stats {
			if future {
				b.WriteString(fmt.Sprintf(`<div title="%s" class="f"></div>`, stat.Day))
				continue
			}

			if stat.Day == today {
				future = true
			}

			h := math.Round(float64(stat.Daily) / float64(max) / 0.01)
			st := ""
			if h > 0 {
				hu := math.Round(float64(stat.DailyUnique) / float64(max) / 0.01)
				st = fmt.Sprintf(` style="height:%.0f%%" data-u="%.0f%%"`, h, hu)
			}

			b.WriteString(fmt.Sprintf(`<div%s title="%s|%s|%s"></div>`,
				st, stat.Day, zhttp.Tnformat(stat.Daily, site.Settings.NumberFormat),
				zhttp.Tnformat(stat.DailyUnique, site.Settings.NumberFormat)))
		}

	// Hourly view.
	case false:
		hour := now.Hour()
		for i, stat := range stats {
			for shour, s := range stat.Hourly {
				if future {
					b.WriteString(fmt.Sprintf(`<div title="%s|%[2]d:00|%[2]d:59" class="f"></div>`,
						stat.Day, shour))
					continue
				}

				if stat.Day == today && shour > hour {
					if i == len(stats)-1 { // Don't display future if end date is today.
						break
					}
					future = true
				}

				h := math.Round(float64(s) / float64(max) / 0.01)
				st := ""
				if h > 0 {
					hu := math.Round(float64(stat.HourlyUnique[shour]) / float64(max) / 0.01)
					st = fmt.Sprintf(` style="height:%.0f%%" data-u="%.0f%%"`, h, hu)
				}
				b.WriteString(fmt.Sprintf(`<div%s title="%s|%[3]d:00|%[3]d:59|%s|%s"></div>`,
					st, stat.Day, shour,
					zhttp.Tnformat(s, site.Settings.NumberFormat),
					zhttp.Tnformat(stat.HourlyUnique[shour], site.Settings.NumberFormat)))
			}
		}
	}

	return template.HTML(b.String())
}

func HorizontalChart(ctx context.Context, stats Stats, total, parentTotal int, cutoff float32, link, other bool) template.HTML {
	tag := "p"
	if link {
		tag = "a"
	}

	totalPerc := float32(0.0)
	var b strings.Builder
	for _, s := range stats {
		// TODO: not sure how to display this; doing it in two colours doesn't
		// make much sense, and neither does always displaying unique. Maybe a
		// checkbox to toggle? Or two bars?
		perc := float32(s.Count) / float32(total) * 100
		if parentTotal > 0 {
			perc = float32(s.Count) / float32(parentTotal) * 100
		}
		if perc < cutoff { // Group as "Other" later.
			continue
		}
		totalPerc += perc

		browser := s.Name
		if browser == "" {
			browser = "(unknown)"
		}

		title := fmt.Sprintf("%s: %.1f%% – %s visits; %s pageviews",
			template.HTMLEscapeString(browser), perc,
			zhttp.Tnformat(s.CountUnique, MustGetSite(ctx).Settings.NumberFormat),
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

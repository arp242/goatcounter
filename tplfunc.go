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
	zhttp.FuncMap["beforeSize"] = func(createdAt time.Time) bool { return createdAt.Before(ss) }
	zhttp.FuncMap["beforeLoc"] = func(createdAt time.Time) bool { return createdAt.Before(sl) }

	// Implemented as function for performance.
	zhttp.FuncMap["bar_chart"] = BarChart
	zhttp.FuncMap["horizontal_chart"] = HorizontalChart
}

func BarChart(ctx context.Context, stats []Stat, max int) template.HTML {
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
				stat.Day, shour, zhttp.Tnformat(s, MustGetSite(ctx).Settings.NumberFormat), inner))
		}
	}

	return template.HTML(b.String())
}

func HorizontalChart(ctx context.Context, stats Stats, total, parentTotal int, cutoff float32, link bool) template.HTML {
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
	if totalPerc < 100 {
		b.WriteString(fmt.Sprintf(
			`<%[2]s href="#_" title="(other): %.1[1]f%%" class="other"><small>(other)</small> <span style="width: %[1]f%%">%.1[1]f%%</span></%[2]s>`,
			100-totalPerc, tag))
	}

	return template.HTML(b.String())
}

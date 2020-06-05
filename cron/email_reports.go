// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/now"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
)

var el = zlog.Module("email-report")

// EmailReports sends email reports for sites that have this configured.
func EmailReports(ctx context.Context) error {
	db := zdb.MustGet(ctx)

	var (
		ids []int64
		err error
	)
	if cfg.PgSQL {
		err = db.SelectContext(ctx, &ids,
			`select id from sites where settings->>'email_reports'::varchar != '0'`)
	} else {
		// Bit ugly to avoid having to depend on SQLite's JSON extensions, which
		// requires "-tags sqlite_json" to be added to every go run/build/test
		// command, which is annoying and prone to errors.
		//
		// TODO: find a good solution for this, for example by automatically
		// setting this tag and/or -DSQLITE_ENABLE_JSON1 somehow.
		//
		// err = db.SelectContext(ctx, &ids,
		//    `select id from sites where json_extract(settings, '$.email_reports') != '0'`)

		var allSites goatcounter.Sites
		err = allSites.List(ctx)
		if err == nil {
			for _, s := range allSites {
				if s.Settings.EmailReports != 0 {
					ids = append(ids, s.ID)
				}
			}
		}
	}
	if err != nil {
		return fmt.Errorf("cron.emailReports get sites: %w", err)
	}

	var sites goatcounter.Sites
	err = sites.ListIDs(ctx, ids...)
	if err != nil {
		return fmt.Errorf("cron.emailReports: %w", err)
	}

	for _, s := range sites {
		el.Debugf("running for site %d; setting: %d", s.ID, s.Settings.EmailReports)

		var user goatcounter.User
		err = user.BySite(ctx, s.ID)
		if err != nil {
			zlog.Error(err)
			continue
		}

		text, html, subject, err := report(ctx, s, user.LastReportAt)
		if err != nil {
			zlog.Field("site", s.ID).Errorf("cron.emailReports: %w", err)
			continue
		}
		if text == nil {
			el.Debug("no text: bailing")
			continue
		}

		err = blackmail.Send(subject,
			blackmail.From("GoatCounter report", cfg.EmailFrom),
			append(blackmail.To(user.Email), blackmail.Cc(s.Settings.EmailReportsCc...)...),
			blackmail.BodyText(text),
			blackmail.BodyHTML(html))
		if err != nil {
			zlog.Error(err)
			continue
		}

		_, err = db.ExecContext(ctx, `update users set last_report_at=$1 where site=$2`,
			goatcounter.Now().Format(zdb.Date), s.ID)
		if err != nil {
			zlog.Error(err)
		}
	}

	return nil
}

type templateArgs struct {
	DisplayDate                  string
	Site                         *goatcounter.Site
	Start, End                   time.Time
	FirstTime, NewFeature        bool
	PeriodName, PeriodNamely     string
	TextPagesTable, TextRefTable template.HTML
	Pages, Refs                  goatcounter.HitStats
	Diffs                        []string
}

// Note: this has the assumption that the current day is the first day of
// the new period. For "daily" this is the next day, for "weekly" this is
// the first day of the week, etc.
func report(ctx context.Context, s goatcounter.Site, lastReport *time.Time) ([]byte, []byte, string, error) {
	s.Settings.Limits.Page = 10
	s.Settings.Limits.Ref = 10
	ctx = goatcounter.WithSite(ctx, &s)

	ln := goatcounter.Now().In(s.Settings.Timezone.Location)
	n := now.Now{
		Time: time.Date(ln.Year(), ln.Month(), ln.Day()-1, 12, 0, 0, 0, s.Settings.Timezone.Location),
		Config: &now.Config{
			TimeLocation: s.Settings.Timezone.Location,
			WeekStartDay: map[bool]time.Weekday{true: time.Sunday, false: time.Monday}[s.Settings.SundayStartsWeek],
		},
	}

	args := templateArgs{Site: &s, End: n.EndOfDay()}

	switch s.Settings.EmailReports.Int() {
	case goatcounter.EmailReportDaily:
		args.PeriodName = "day"
		args.PeriodNamely = "daily"
		args.Start = n.BeginningOfDay()
		args.End = n.EndOfDay()
		args.DisplayDate = args.Start.Format("Mon January ") + ordinal(args.Start.Day())
	case goatcounter.EmailReportBiWeekly:
		args.PeriodName = "biweek"
		args.PeriodNamely = "biweekly"
		args.Start = n.BeginningOfWeek().Add(-7 * 24 * time.Hour)
		args.End = now.New(args.Start).EndOfWeek()

		args.DisplayDate = fmt.Sprintf("%s %s to %s %s",
			args.Start.Format("Mon Jan"), ordinal(args.Start.Day()),
			args.End.Format("Mon Jan"), ordinal(args.End.Day()))
	case goatcounter.EmailReportMonthly:
		args.PeriodName = "month"
		args.PeriodNamely = "monthly"
		args.Start = n.BeginningOfMonth()
		args.End = now.New(args.Start).EndOfMonth()

		args.DisplayDate = args.Start.Format("January")

	case goatcounter.EmailReportOnce:
		args.FirstTime = true
		fallthrough
	case goatcounter.EmailReportInit:
		args.PeriodName = "first-time"
		args.NewFeature = true
		fallthrough
	case goatcounter.EmailReportWeekly:
		if args.PeriodName == "" {
			args.PeriodName = "week"
		}
		args.PeriodNamely = "weekly"
		args.Start = n.BeginningOfWeek()
		args.End = now.New(args.Start).EndOfWeek()
		args.DisplayDate = fmt.Sprintf("%s %s to %s %s",
			args.Start.Format("Mon Jan"), ordinal(args.Start.Day()),
			args.End.Format("Mon Jan"), ordinal(args.End.Day()))
	default:
		return nil, nil, "", errors.Errorf(
			"cron.report: invalid value for report: %s", s.Settings.EmailReports)
	}

	el.Debugf("%q: start: %s; end: %s", args.PeriodName, args.Start, args.End)

	// Subject is unique mostly to prevent gmail from grouping messages by
	// subject, which in turn results in it hidding content :-/ I don't get
	// why people use gmail 🤷
	subject := fmt.Sprintf("Your %s GoatCounter report for %s", args.PeriodNamely, args.DisplayDate)

	// Get hit stats.
	{
		var pages goatcounter.HitStats
		_, _, td, tud, _, err := pages.List(ctx, args.Start, args.End, "", nil, true)
		if err != nil {
			return nil, nil, "", fmt.Errorf("cron.report: %w", err)
		}

		// No pages: nothing to report I guess?
		if len(pages) == 0 {
			return nil, nil, "", nil
		}

		var totals goatcounter.HitStat
		_, err = totals.Totals(ctx, args.Start, args.End, "", true)
		if err != nil {
			return nil, nil, "", fmt.Errorf("cron.report: %w", err)
		}
		args.Pages = pages

		diffs, err := (goatcounter.HitStats{}).DiffTotal(ctx, args.Start, args.End, pages.Paths())
		if err != nil {
			return nil, nil, "", fmt.Errorf("cron.report: %w", err)
		}

		diffStr := make([]string, len(diffs))
		for i := range diffs {
			switch {
			case math.IsInf(float64(diffs[i]), 0):
				diffStr[i] = "new"
			case diffs[i] < 0:
				diffStr[i] = fmt.Sprintf("%+.0f%%", diffs[i])
			default:
				diffStr[i] = fmt.Sprintf("%.0f%%", diffs[i])
			}
		}
		args.Diffs = diffStr

		b := new(strings.Builder)
		fmt.Fprintf(b, "    %-25s  %9s  %9s  %7s\n\n", "Path", "Visits", "Pageviews", "Growth")

		for i, p := range pages {
			path := p.Path
			if p.Event {
				path += " (e)"
			}

			fmt.Fprintf(b, "    %-25s  %9s  %9s  %7s\n",
				template.HTMLEscapeString(zstring.Left(path, 24)),
				zhttp.Tnformat(p.CountUnique, s.Settings.NumberFormat),
				zhttp.Tnformat(p.Count, s.Settings.NumberFormat),
				diffStr[i])

			//pages[i].Path = zstring.Left(pages[i].Path, 24)
		}
		fmt.Fprintf(b, "\n    %-25s  %9s  %9s\n",
			"Displayed",
			zhttp.Tnformat(tud, s.Settings.NumberFormat),
			zhttp.Tnformat(td, s.Settings.NumberFormat))
		fmt.Fprintf(b, "    %-25s  %9s  %9s  %+6d%%\n",
			"Total",
			zhttp.Tnformat(totals.CountUnique, s.Settings.NumberFormat),
			zhttp.Tnformat(totals.Count, s.Settings.NumberFormat),
			12)

		args.TextPagesTable = template.HTML(b.String())
	}

	// Get ref stats.
	{
		var refs goatcounter.HitStats
		_, err := refs.ListAllRefs(ctx, args.Start, args.End, 0)
		if err != nil {
			return nil, nil, "", fmt.Errorf("cron.report: %w", err)
		}
		args.Refs = refs

		b := new(strings.Builder)
		fmt.Fprintf(b, "    %-34s  %9s  %9s\n\n", "Referrer", "Visits", "Pageviews")

		for _, r := range refs {
			path := r.Path
			if path == "" {
				path = "(no data)"
			}
			fmt.Fprintf(b, "    %-34s  %9s  %9s\n",
				template.HTMLEscapeString(zstring.Left(path, 33)),
				zhttp.Tnformat(r.CountUnique, s.Settings.NumberFormat),
				zhttp.Tnformat(r.Count, s.Settings.NumberFormat))

			//refs[i].Path = zstring.Left(refs[i].Path, 33)
		}

		args.TextRefTable = template.HTML(b.String())
	}

	text, err := zhttp.ExecuteTpl("email_report.gotxt", args)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cron.report text: %w", err)
	}
	html, err := zhttp.ExecuteTpl("email_report.gohtml", args)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cron.report html: %w", err)
	}

	return text, html, subject, nil
}

func ordinal(x int) string {
	suffix := "th"
	switch x % 10 {
	case 1:
		if x%100 != 11 {
			suffix = "st"
		}
	case 2:
		if x%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if x%100 != 13 {
			suffix = "rd"
		}
	}
	return strconv.Itoa(x) + suffix
}

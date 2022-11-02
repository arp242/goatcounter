// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"strings"

	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztime"
	"zgo.at/ztpl"
	"zgo.at/ztpl/tplfunc"
)

var el = zlog.Module("email-report")

// EmailReports sends email reports for sites that have this configured.
func EmailReports(ctx context.Context) error {
	users, err := reportUsers(ctx)
	if err != nil {
		return errors.Errorf("cron.EmailReports: %w", err)
	}

	now := ztime.Now().UTC()
	for _, user := range users {
		var site goatcounter.Site
		err := site.ByID(ctx, user.Site)
		if err != nil {
			el.Error(err)
			continue
		}

		if user.LastReportAt.IsZero() {
			return fmt.Errorf("cron.EmailReports: user=%d: LastReportAt is zero; this should never happen", user.ID)
		}
		if user.LastReportAt.After(now) {
			return fmt.Errorf("cron.EmailReports: user=%d: LastReportAt is after the current time; this should never happen", user.ID)
		}

		rng := user.EmailReportRange().UTC()
		if rng.End.After(now) || rng.IsZero() {
			continue
		}

		text, html, subject, err := reportText(ctx, site, user)
		if err != nil {
			return fmt.Errorf("cron.EmailReports: user=%d: %w", user.ID, err)
		}
		if text == nil {
			el.Debug("no text: bailing")
			continue
		}

		err = blackmail.Send(subject,
			blackmail.From("GoatCounter reports", goatcounter.Config(ctx).EmailFrom),
			blackmail.To(user.Email),
			blackmail.BodyText(text),
			blackmail.BodyHTML(html))
		if err != nil {
			zlog.Error(err)
			continue
		}

		err = zdb.Exec(ctx, `update users set last_report_at=$1 where user_id=$2`, ztime.Now(), user.ID)
		if err != nil {
			zlog.Error(err)
		}
	}
	return nil
}

// Get list of all users to send reports for.
func reportUsers(ctx context.Context) (goatcounter.Users, error) {
	query := `
		select users.* from users
		join sites using(site_id)
		where json_extract(users.settings, '$.email_reports') != ? and sites.state = ?`
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		query = `
			select users.* from users
			join sites using(site_id)
			where users.settings->>'email_reports'::text != ? and sites.state = ?`
	}

	var users goatcounter.Users
	err := zdb.Select(ctx, &users, query, goatcounter.EmailReportNever, goatcounter.StateActive)
	return users, errors.Wrap(err, "get users")
}

type templateArgs struct {
	Context context.Context
	Site    goatcounter.Site
	User    goatcounter.User
	Pages   goatcounter.HitLists
	Total   goatcounter.HitList
	Refs    goatcounter.HitStats

	DisplayDate                  string
	TextPagesTable, TextRefTable template.HTML

	Diffs []string
}

func reportText(ctx context.Context, site goatcounter.Site, user goatcounter.User) (text, html []byte, subject string, err error) {
	ctx = goatcounter.WithSite(ctx, &site)
	rng := user.EmailReportRange().UTC()

	args := templateArgs{
		Context:     ctx,
		Site:        site,
		User:        user,
		DisplayDate: fmt.Sprintf("%s ", rng.Start.Format(user.Settings.DateFormat)),
	}
	// TODO: ztime.Range.String() prints "relative" dates such as "yesterday"
	// and "last week"; this is nice in some cases, but not so nice in others
	// (such as here). Should have two functions for this.
	if user.Settings.EmailReports != goatcounter.EmailReportDaily {
		args.DisplayDate += " – " + rng.End.Format(user.Settings.DateFormat)
	}
	// TODO: no locale on context here.
	subject = fmt.Sprintf("Your GoatCounter report for %s", args.DisplayDate)

	{ // Get overview of paths.
		_, _, _, err := args.Pages.List(ctx, rng, nil, nil, 10, true)
		if err != nil {
			return nil, nil, "", err
		}

		if len(args.Pages) == 0 { /// No pages: don't bother sending out anything.
			return nil, nil, "", nil
		}

		_, err = args.Total.Totals(ctx, rng, nil, true, true)
		if err != nil {
			return nil, nil, "", err
		}

		d := -rng.End.Sub(rng.Start)
		prev := ztime.NewRange(rng.Start.Add(d)).To(rng.End.Add(d))
		diffs, err := args.Pages.Diff(ctx, rng, prev)
		if err != nil {
			return nil, nil, "", err
		}

		diffStr := make([]string, len(args.Pages))
		for i := range diffs {
			switch {
			case math.IsInf(diffs[i], 0):
				diffStr[i] = "(new)"
			case diffs[i] < 0:
				diffStr[i] = fmt.Sprintf("%+.0f%%", diffs[i])
			default:
				diffStr[i] = fmt.Sprintf("%.0f%%", diffs[i])
			}
		}
		args.Diffs = diffStr

		b := new(strings.Builder)
		fmt.Fprintf(b, "    %-36s  %9s  %7s\n", "Path", "Visitors", "Growth")
		b.WriteString("    " + strings.Repeat("-", 56) + "\n")
		b.WriteByte('\n')
		for i, p := range args.Pages {
			path := p.Path
			if p.Event {
				path += " (e)"
			}

			fmt.Fprintf(b, "    %-36s  %9s  %7s\n",
				template.HTMLEscapeString(zstring.ElideLeft(path, 35)),
				tplfunc.Number(p.CountUnique, user.Settings.NumberFormat),
				diffStr[i])
		}
		args.TextPagesTable = template.HTML(b.String())
	}

	{ // Get overview of refs.
		err := args.Refs.ListTopRefs(ctx, rng, nil, 10, 0)
		if err != nil {
			return nil, nil, "", err
		}

		b := new(strings.Builder)
		fmt.Fprintf(b, "    %-45s  %9s\n", "Referrer", "Visitors")
		b.WriteString("    " + strings.Repeat("-", 56) + "\n")
		for _, r := range args.Refs.Stats {
			path := r.Name
			if path == "" {
				path = "(no data)"
			}
			fmt.Fprintf(b, "    %-45s  %9s\n",
				template.HTMLEscapeString(zstring.ElideLeft(path, 44)),
				tplfunc.Number(r.CountUnique, user.Settings.NumberFormat))
			//refs[i].Path = zstring.ElideLeft(refs[i].Path, 44)
		}
		args.TextRefTable = template.HTML(b.String())
	}

	text, err = ztpl.ExecuteBytes("email_report.gotxt", args)
	if err != nil {
		return nil, nil, "", errors.Errorf("cron.report text: %w", err)
	}
	html, err = ztpl.ExecuteBytes("email_report.gohtml", args)
	if err != nil {
		return nil, nil, "", errors.Errorf("cron.report html: %w", err)
	}

	return text, html, subject, nil
}

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
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/zdb"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztime"
	"zgo.at/ztpl"
	"zgo.at/ztpl/tplfunc"
)

var el = log.Module("email-report")

// EmailReports sends email reports for sites that have this configured.
func EmailReports(ctx context.Context) error {
	users, err := reportUsers(ctx)
	if err != nil {
		return errors.Errorf("cron.emailReports: %w", err)
	}

	now := ztime.Now(ctx).UTC()
	for _, user := range users {
		var site goatcounter.Site
		err := site.ByID(ctx, user.Site)
		if err != nil {
			el.Error(ctx, err)
			continue
		}

		if user.LastReportAt.IsZero() {
			return fmt.Errorf("cron.emailReports: user=%d: LastReportAt is zero; this should never happen", user.ID)
		}
		if user.LastReportAt.After(now) {
			return fmt.Errorf("cron.emailReports: user=%d: LastReportAt is after the current time; this should never happen", user.ID)
		}

		rng := user.EmailReportRange(ctx).UTC()
		if rng.End.After(now) || rng.IsZero() {
			continue
		}

		text, html, subject, err := reportText(ctx, site, user)
		if err != nil {
			return fmt.Errorf("cron.emailReports: user=%d: %w", user.ID, err)
		}
		if text == nil {
			el.Debug(ctx, "no text: bailing")
			continue
		}

		err = blackmail.Get(ctx).Send(subject,
			blackmail.From("GoatCounter reports", goatcounter.Config(ctx).EmailFrom),
			blackmail.To(user.Email),
			blackmail.HeadersAutoreply(),
			blackmail.BodyText(text),
			blackmail.BodyHTML(html))
		if err != nil {
			el.Error(ctx, err)
			continue
		}

		err = zdb.Exec(ctx, `update users set last_report_at=$1 where user_id=$2`, ztime.Now(ctx), user.ID)
		if err != nil {
			el.Error(ctx, err)
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

type (
	reportArgs struct {
		Context     context.Context
		Account     goatcounter.Site
		User        goatcounter.User
		DisplayDate string
		Sites       []reportArgsSite
	}
	reportArgsSite struct {
		URL                          string
		Pages                        goatcounter.HitLists
		Total                        goatcounter.HitList
		Refs                         goatcounter.HitStats
		TextPagesTable, TextRefTable template.HTML
		Diffs                        []string
	}
)

func reportText(ctx context.Context, account goatcounter.Site, user goatcounter.User) (text, html []byte, subject string, err error) {
	var sites goatcounter.Sites
	err = sites.ForAccount(ctx, account.ID)
	if err != nil {
		return nil, nil, "", err
	}

	rng := user.EmailReportRange(ctx).UTC()
	args := reportArgs{
		Context:     ctx,
		Account:     account,
		User:        user,
		DisplayDate: rng.Start.Format(user.Settings.DateFormat),
	}
	if user.Settings.EmailReports != goatcounter.EmailReportDaily {
		args.DisplayDate += " – " + rng.End.Format(user.Settings.DateFormat)
	}
	subject = fmt.Sprintf("Your GoatCounter report for %s", args.DisplayDate)

	for _, s := range sites {
		sa, err := reportTextSite(ctx, s, user)
		if err != nil {
			return nil, nil, "", err
		}
		if len(sa.Pages) > 0 {
			args.Sites = append(args.Sites, sa)
		}
	}
	if len(args.Sites) == 0 {
		return nil, nil, "", nil
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

func reportTextSite(ctx context.Context, site goatcounter.Site, user goatcounter.User) (reportArgsSite, error) {
	ctx = goatcounter.WithSite(ctx, &site)
	var (
		args = reportArgsSite{URL: site.URL(ctx)}
		rng  = user.EmailReportRange(ctx).UTC()
	)

	{ // Get overview of paths.
		_, _, err := args.Pages.List(ctx, rng, goatcounter.PathFilter{}, nil, 10, goatcounter.GroupDaily)
		if err != nil {
			return args, err
		}

		if len(args.Pages) == 0 { /// No pages: don't bother sending out anything.
			return args, nil
		}

		_, err = args.Total.Totals(ctx, rng, goatcounter.PathFilter{}, goatcounter.GroupDaily, true)
		if err != nil {
			return args, err
		}

		d := -rng.End.Sub(rng.Start)
		prev := ztime.NewRange(rng.Start.Add(d)).To(rng.End.Add(d))
		diffs, err := args.Pages.Diff(ctx, rng, prev)
		if err != nil {
			return args, err
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
		for i, p := range args.Pages {
			path := p.Path
			if p.Event {
				path += " (e)"
			}

			fmt.Fprintf(b, "    %-36s  %9s  %7s\n",
				template.HTMLEscapeString(zstring.ElideLeft(path, 35)),
				tplfunc.Number(p.Count, user.Settings.NumberFormat),
				diffStr[i])
		}
		args.TextPagesTable = template.HTML(b.String())
	}

	{ // Get overview of refs.
		err := args.Refs.ListTopRefs(ctx, rng, goatcounter.PathFilter{}, 10, 0)
		if err != nil {
			return args, err
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
				tplfunc.Number(r.Count, user.Settings.NumberFormat))
			//refs[i].Path = zstring.ElideLeft(refs[i].Path, 44)
		}
		args.TextRefTable = template.HTML(b.String())
	}

	return args, nil
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"io/fs"
	"math"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/russross/blackfriday/v2"
	"zgo.at/errors"
	"zgo.at/zcache"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zhttp/ztpl/tplfunc"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/zstring"
	"zgo.at/zvalidate"
)

func init() {
	tplfunc.Add("base32", base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString)
	tplfunc.Add("validate", zvalidate.TemplateError)
	tplfunc.Add("has_errors", zvalidate.TemplateHasErrors)
	tplfunc.Add("error_code", func(err error) string { return zhttp.UserErrorCode(err) })
	tplfunc.Add("parent_site", func(ctx context.Context, id *int64) string {
		var s Site
		err := s.ByID(ctx, *id)
		if err != nil {
			zlog.Error(err)
			return ""
		}
		return s.URL(ctx)
	})

	// Implemented as function for performance.
	tplfunc.Add("bar_chart", barChart)
	tplfunc.Add("text_chart", textChart)
	tplfunc.Add("horizontal_chart", HorizontalChart)

	var mdCache = zcache.New(zcache.NoExpiration, zcache.NoExpiration)
	tplfunc.Add("markdown", func(file string, scope interface{}) template.HTML {
		ctx := reflect.ValueOf(scope).FieldByName("Context").Elem().Interface().(context.Context)
		if !Config(ctx).Dev {
			if c, ok := mdCache.Get(file); ok {
				return c.(template.HTML)
			}
		}

		fsys, err := zfs.EmbedOrDir(Templates, "tpl", Config(ctx).Dev)
		if err != nil {
			panic(err)
		}
		fsys, err = zfs.SubIfExists(fsys, "tpl/code")
		if err != nil {
			panic(err)
		}
		f, err := fs.ReadFile(fsys, file+".markdown")
		if err != nil {
			panic(err)
		}

		md := renderMarkdown(f)
		t, err := template.New("md").Parse(string(md))
		if err != nil {
			panic(err)
		}

		// {{if .FromWWW}}
		// 	<p class="flash flash-i">Note: replace <code>{{.SiteURL}}</code> with the URL
		// 	to your actual site in the examples below. This will be done automatically if
		// 	you view the docs linked from your site in the top-right corner.</p>
		// {{end}}
		t = template.Must(t.New("code").Parse(`&lt;script data-goatcounter="{{.SiteURL}}/count"` + "\n" +
			`        async src="//{{.CountDomain}}/count.js"&gt;&lt;/script&gt;`))
		t = template.Must(t.New("sh_header").Parse(`#!/bin/sh` + "\n" +
			`token=\[your api token]` + "\n" +
			`api=https://\[my code].goatcounter.com/api/v0` + "\n" +
			`curl() {` + "\n" +
			`    \command curl \` + "\n" +
			`        -H 'Content-Type: application/json' \` + "\n" +
			`        -H "Authorization: Bearer $token" \` + "\n" +
			`        "$@"` + "\n" +
			`}`))

		html := new(bytes.Buffer)
		err = t.ExecuteTemplate(html, "md", scope)
		if err != nil {
			panic(err)
		}

		mdCache.SetDefault(file, template.HTML(html.String()))
		return template.HTML(html.String())
	})

	type x struct {
		href, label string
		items       []x
	}
	links := []x{
		{label: "Basics", items: []x{
			{href: "start", label: "Getting started"},
			// {href: "wordpress", label: "WordPress"},
			{href: "visitor-counter", label: "Visitor counter"},
			{href: "events", label: "Events"},
			{href: "csp", label: "Content-Security-Policy"},
			{href: "js", label: "JavaScript API"}}},
		{label: "Other ways to get data in GoatCounter", items: []x{
			{href: "pixel", label: "Tracking pixel"},
			{href: "logfile", label: "Server logfiles"},
			{href: "backend", label: "From app backend or other sources"}}},
		{label: "How can I…", items: []x{
			{href: "skip-dev", label: "Prevent tracking my own pageviews?"},
			{href: "path", label: "Control the path that's sent to GoatCounter?"},
			{href: "modify", label: "Change data before it's send to GoatCounter?"},
			{href: "domains", label: "Track multiple domains/sites?"},
			{href: "spa", label: "Add GoatCounter to a SPA?"},
			{href: "beacon", label: "Use navigator.sendBeacon?"},
			{href: "countjs-versions", label: "count.js versions and SRI"},
			{href: "countjs-host", label: "Host count.js somewhere else?"}}},
		{label: "Other", items: []x{
			// TODO: add "adblock" page
			// TODO: add "campiagns page"; link in "settings_main".
			{href: "export", label: "Exports"},
			{href: "api", label: "API"}}},
	}
	tplfunc.Add("code_nav", func(active string) template.HTML {
		var (
			dropdown = new(strings.Builder)
			list     = new(strings.Builder)
			w        func([]x)
			e        = template.HTMLEscapeString
		)

		w = func(l []x) {
			for _, ll := range l {
				if len(ll.items) > 0 {
					fmt.Fprintf(list, `<li><strong>%s</strong><ul>`, e(ll.label))
					fmt.Fprintf(dropdown, `<optgroup label="%s">`, e(ll.label))
					w(ll.items)
					list.WriteString("</ul></li>")
					dropdown.WriteString("</optgroup>")
					continue
				}

				list.WriteString("<li")
				dropdown.WriteString(`<option`)
				if ll.href == active {
					list.WriteString(` class="active"`)
					dropdown.WriteString(` selected`)
				}

				fmt.Fprintf(list, `><a href="%s">%s</a></li>`, e(ll.href), e(ll.label))
				fmt.Fprintf(dropdown, ` value="%s">%s</option>`, e(ll.href), e(ll.label))
			}
		}

		dropdown.WriteString("<select>")
		list.WriteString("<ul>")
		w(links)
		dropdown.WriteString("</select>")
		list.WriteString("</ul>")
		return template.HTML(dropdown.String() + list.String())
	})
	tplfunc.Add("code_hdr", func(active string) template.HTML {
		if active == "404" {
			return "404: Not Found"
		}
		var w func([]x) string
		w = func(l []x) string {
			for _, ll := range l {
				if ll.href == active {
					return strings.TrimRight(ll.label, "?")
				}
				if len(ll.items) > 0 {
					if r := w(ll.items); r != "" {
						return r
					}
				}
			}
			return ""
		}
		return template.HTML(w(links))
	})

	// Override defaults to take user settings in to account.
	tplfunc.Add("tformat", func(t time.Time, fmt string, u User) string {
		if fmt == "" {
			fmt = "2006-01-02"
		}
		return t.In(u.Settings.Timezone.Loc()).Format(fmt)
	})
	tplfunc.Add("nformat", func(n int, u User) string {
		return tplfunc.Number(n, u.Settings.NumberFormat)
	})

	tplfunc.Add("totp_barcode", func(email, s string) template.HTML {
		qrCode, err := qr.Encode(
			fmt.Sprintf("otpauth://totp/GoatCounter:%s?secret=%s&issuer=GoatCounter", email, s),
			qr.M, qr.Auto)
		if err != nil {
			zlog.Error(errors.Wrap(err, "encoding QR code"))
			return template.HTML("Error generating the QR code; this has been logged for investigation.")
		}

		qrCode, err = barcode.Scale(qrCode, 200, 200)
		if err != nil {
			zlog.Error(errors.Wrap(err, "scaling QR code"))
			return template.HTML("Error generating the QR code; this has been logged for investigation.")
		}

		buf := bytes.NewBufferString("data:image/png;base64,")
		err = png.Encode(base64.NewEncoder(base64.StdEncoding, buf), qrCode)
		if err != nil {
			zlog.Error(errors.Wrap(err, "encoding QR code as PNG"))
			return template.HTML("Error generating the QR code; this has been logged for investigation.")
		}

		return template.HTML(fmt.Sprintf(
			`<img alt="TOTP Secret Barcode" title="TOTP Secret Barcode" src="%s">`,
			buf.String()))
	})
}

var (
	reTemplate    = regexp.MustCompile(`{{(\w+ .*?)(?:&ldquo;|&quot;)(.+?)(?:&rdquo;|&quot;)(.*?)}}`)
	reHeaderLinks = regexp.MustCompile(`<h([2-6]) id="(.*?)">(.*?)<\/h[2-6]>`)

	markdownOpt = []blackfriday.Option{
		blackfriday.WithNoExtensions(),
		blackfriday.WithExtensions(blackfriday.NoExtensions |
			blackfriday.NoIntraEmphasis |
			blackfriday.Tables |
			blackfriday.FencedCode |
			blackfriday.Strikethrough |
			blackfriday.SpaceHeadings |
			blackfriday.HeadingIDs |
			blackfriday.AutoHeadingIDs |
			blackfriday.BackslashLineBreak |
			blackfriday.Autolink |
			blackfriday.Footnotes,
		),

		blackfriday.WithRenderer(blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{Flags: blackfriday.HTMLFlagsNone |
			blackfriday.Smartypants |
			blackfriday.SmartypantsFractions |
			blackfriday.SmartypantsDashes |
			blackfriday.FootnoteReturnLinks,
		})),
	}
)

// Render Markdown with our flags.
//
// This also hacks around things a bit to ensure that template directives aren't
// mangled.
func renderMarkdown(in []byte) []byte {
	md := blackfriday.Run(in, markdownOpt...)
	md = reTemplate.ReplaceAll(md, []byte(`{{$1 "$2"$3}}`))
	md = reHeaderLinks.ReplaceAll(md, []byte(`<h$1 id="$2">$3 <a href="#$2"></a></h$1>`))
	return md
}

var textSymbols = []rune{
	'\u2007', // FIGURE SPACE; this one has the closest width to the blocks.
	'▁',      // U+2581 LOWER ONE EIGHTH BLOCK
	'▂',      // U+2582 LOWER ONE QUARTER BLOCK
	'▃',      // U+2583 LOWER THREE EIGHTHS BLOCK
	'▄',      // U+2584 LOWER HALF BLOCK
	'▅',      // U+2585 LOWER FIVE EIGHTHS BLOCK
	'▆',      // U+2586 LOWER THREE QUARTERS BLOCK
	'▇',      // U+2587 LOWER SEVEN EIGHTHS BLOCK
	'█',      // U+2588 FULL BLOCK
}

func textChart(ctx context.Context, stats []HitListStat, max int, daily bool) template.HTML {
	_, chunked := ChunkStat(stats)
	symb := make([]rune, 0, 12)
	for _, chunk := range chunked {
		perc := int(math.Floor(float64(chunk) / float64(max) * 100))
		symb = append(symb, textSymbols[perc/12])
	}
	return template.HTML(symb)
}

func barChart(ctx context.Context, stats []HitListStat, max int, daily bool) template.HTML {
	user := MustGetUser(ctx)
	now := Now().In(user.Settings.Timezone.Loc())
	today := now.Format("2006-01-02")

	var (
		future bool
		b      strings.Builder
	)
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
				st, stat.Day, tplfunc.Number(stat.Daily, user.Settings.NumberFormat),
				tplfunc.Number(stat.DailyUnique, user.Settings.NumberFormat)))
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
					tplfunc.Number(s, user.Settings.NumberFormat),
					tplfunc.Number(stat.HourlyUnique[shour], user.Settings.NumberFormat)))
			}
		}
	}

	return template.HTML(b.String())
}

func HorizontalChart(ctx context.Context, stats HitStats, total, pageSize int, link, paginate bool) template.HTML {
	if total == 0 {
		return `<em>Nothing to display</em>`
	}

	var (
		displayed int
		b         strings.Builder
	)
	b.WriteString(`<div class="rows">`)
	for i, s := range stats.Stats {
		if pageSize > 0 && i > pageSize {
			break
		}
		displayed += s.CountUnique

		var (
			p    = float64(s.CountUnique) / float64(total) * 100
			perc string
		)
		switch {
		case p == 0:
			perc = "0%"
		case p < .5:
			perc = fmt.Sprintf("%.1f%%", p)[1:]
		default:
			perc = fmt.Sprintf("%.0f%%", math.Round(p))
		}

		name := template.HTMLEscapeString(s.Name)
		if name == "" {
			name = "(unknown)"
		}
		class := ""
		if name == "(unknown)" || (s.RefScheme != nil && string(*s.RefScheme) == *RefSchemeGenerated) {
			class = "generated"
		}
		visit := ""
		if !link && s.RefScheme != nil && string(*s.RefScheme) == *RefSchemeHTTP {
			visit = fmt.Sprintf(
				`<sup class="go"><a rel="noopener" target="_blank" href="http://%s">visit</a></sup>`,
				name)
		}

		if strings.HasPrefix(name, "twitter.com/search?q=") {
			if i := strings.LastIndex(name, "t.co%2F"); i > -1 {
				name = "Twitter link: t.co/" + name[i+7:]
			}
		}

		ename := zstring.ElideCenter(name, 76)
		var ref string
		if link {
			ref = fmt.Sprintf(`<a href="#" class="load-detail">`+
				`<span class="bar" style="width: %s"></span>`+
				`<span class="bar-c"><span class="cutoff">%s</span> %s</span></a>`, perc, ename, visit)
		} else {
			ref = fmt.Sprintf(`<span class="bar" style="width: %s"></span>`+
				`<span class="bar-c"><span class="cutoff">%s</span> %s</span>`, perc, ename, visit)
		}

		id := s.ID
		if id == "" {
			id = name
		}
		b.WriteString(fmt.Sprintf(`
			<div class="%[1]s" data-name="%[2]s">
				<span class="col-count col-perc">%[3]s</span>
				<span class="col-name">%[4]s</span>
				<span class="col-count">%[5]s</span>
			</div>`,
			class, id, perc, ref,
			tplfunc.Number(s.CountUnique, MustGetUser(ctx).Settings.NumberFormat)))
	}
	b.WriteString(`</div>`)

	// Add pagination link.
	if paginate && stats.More {
		b.WriteString(`<a href="#", class="load-more">Show more</a>`)
	}

	return template.HTML(b.String())
}

type (
	TplEmailWelcome struct {
		Context     context.Context
		Site        Site
		User        User
		CountDomain string
	}
	TplEmailForgotSite struct {
		Context context.Context
		Sites   Sites
		Email   string
	}
	TplEmailPasswordReset struct {
		Context context.Context
		Site    Site
		User    User
	}
	TplEmailVerify struct {
		Context context.Context
		Site    Site
		User    User
	}
	TplEmailAddUser struct {
		Context context.Context
		Site    Site
		NewUser User
		AddedBy string
	}
	TplEmailImportError struct {
		Error error
	}
	TplEmailExportDone struct {
		Context context.Context
		Site    Site
		User    User
		Export  Export
	}
	TplEmailImportDone struct {
		Site   Site
		Rows   int
		Errors *errors.Group
	}
)

var E = ztpl.ExecuteBytes

func (t TplEmailWelcome) Render() ([]byte, error)       { return E("email_welcome.gotxt", t) }
func (t TplEmailForgotSite) Render() ([]byte, error)    { return E("email_forgot_site.gotxt", t) }
func (t TplEmailPasswordReset) Render() ([]byte, error) { return E("email_password_reset.gotxt", t) }
func (t TplEmailVerify) Render() ([]byte, error)        { return E("email_verify.gotxt", t) }
func (t TplEmailAddUser) Render() ([]byte, error)       { return E("email_adduser.gotxt", t) }
func (t TplEmailImportError) Render() ([]byte, error)   { return E("email_import_error.gotxt", t) }
func (t TplEmailExportDone) Render() ([]byte, error)    { return E("email_export_done.gotxt", t) }
func (t TplEmailImportDone) Render() ([]byte, error)    { return E("email_import_done.gotxt", t) }

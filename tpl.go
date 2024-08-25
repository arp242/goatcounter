// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

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
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/russross/blackfriday/v2"
	"zgo.at/errors"
	"zgo.at/z18n"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztime"
	"zgo.at/ztpl"
	"zgo.at/ztpl/tplfunc"
	"zgo.at/zvalidate"
)

func init() {
	tplfunc.Add("t", z18n.Thtml)
	tplfunc.Add("tag", z18n.Tag)
	tplfunc.Add("plural", z18n.N)

	// TODO: move to ztpl
	tplfunc.Add("concat", func(sep string, strs ...string) string {
		return strings.Join(strs, sep)
	})
	tplfunc.Add("percentage", func(n, total int) float64 {
		return float64(n) / float64(total) * 100
	})
	tplfunc.Add("ago", func(t time.Time) time.Duration {
		return time.Since(t).Round(time.Second)
	})

	tplfunc.Add("round_duration", func(d time.Duration) time.Duration {
		if d < time.Millisecond {
			return d
		}
		if d < time.Second*10 {
			return d.Round(time.Millisecond * 100)
		}
		return d.Round(time.Second)
	})

	tplfunc.Add("distribute_durations", func(times ztime.Durations, n int) template.HTML {
		p := func(d time.Duration) string {
			return ztime.DurationAs(d.Round(time.Millisecond), time.Millisecond)
		}
		b := new(strings.Builder)

		fmt.Fprintln(b, "\nDistribution:")
		dist := times.Distrubute(n)
		var (
			widthDur, widthNum int
			widthBar           = 100.0
		)
		for _, d := range dist {
			if l := len(p(d.Min())); l > widthDur {
				widthDur = l
			}
			if l := len(strconv.Itoa(d.Len())); l > widthNum {
				widthNum = l
			}
		}

		format := fmt.Sprintf("    ≤ %%%ds ms → %%%dd  %%s %%.1f%%%%\n", widthDur, widthNum)
		l := float64(times.Len())
		for _, h := range dist {
			if h.Len() == 0 {
				continue
			}
			r := int(widthBar / (l / float64(h.Len())))
			perc := float64(h.Len()) / l * 100
			fmt.Fprintf(b, format, p(h.Max()), h.Len(), strings.Repeat("▬", r), perc)
		}

		return template.HTML(b.String())
	})

	tplfunc.Add("ord", func(n int) template.HTML {
		s := "th"
		switch n % 10 {
		case 1:
			if n%100 != 11 {
				s = "st"
			}
		case 2:
			if n%100 != 12 {
				s = "nd"
			}
		case 3:
			if n%100 != 13 {
				s = "rd"
			}
		}
		return template.HTML(strconv.Itoa(n) + "<sup>" + s + "</sup>")
	})

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
	tplfunc.Add("text_chart", textChart)
	tplfunc.Add("horizontal_chart", HorizontalChart)

	tplfunc.Add("markdown", func(file string, scope any) template.HTML {
		ctx := reflect.ValueOf(scope).FieldByName("Context").Elem().Interface().(context.Context)
		fsys, err := zfs.EmbedOrDir(Templates, "tpl", Config(ctx).Dev)
		if err != nil {
			panic(err)
		}
		fsys, err = zfs.SubIfExists(fsys, "tpl/help")
		if err != nil {
			panic(err)
		}
		f, err := fs.ReadFile(fsys, file+".md")
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
			`token="[your api token]"` + "\n" +
			`api="https://[my code].goatcounter.com/api/v0"` + "\n" +
			`curl() {` + "\n" +
			`    \command curl --silent \` + "\n" +
			`        --header 'Content-Type: application/json' \` + "\n" +
			`        --header "Authorization: Bearer $token" \` + "\n" +
			`        "$@"` + "\n" +
			`}`))

		html := new(bytes.Buffer)
		err = t.ExecuteTemplate(html, "md", scope)
		if err != nil {
			panic(err)
		}

		return template.HTML(html.String())
	})

	type x struct {
		href, label string
		items       []x
		gcCom       bool
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
			{href: "skip-path", label: "Prevent tracking specific paths?"},
			{href: "path", label: "Control the path that's sent to GoatCounter?"},
			{href: "modify", label: "Change data before it's sent to GoatCounter?"},
			{href: "domains", label: "Track multiple domains/sites?"},
			{href: "spa", label: "Add GoatCounter to a SPA?"},
			{href: "campaigns", label: "Track campaigns?"},
			{href: "countjs-versions", label: "Use SRI with count.js?"},
			{href: "countjs-host", label: "Host count.js somewhere else?"},
			{href: "frame", label: "Embed GoatCounter in a frame?"}}},
		{label: "Other", items: []x{
			// TODO: add "adblock" page
			// TODO: add "campiagns page"; link in "settings_main".
			{href: "export", label: "Export format"},
			{href: "sessions", label: "Sessions and visitors"},
			{href: "api", label: "API"},
			{href: "faq", label: "FAQ"},
			{href: "translating", label: "Translating GoatCounter"}}},
		{label: "Legal", items: []x{
			{href: "gdpr", label: "GDPR consent notices"},
			{href: "terms", label: "Terms of use", gcCom: true},
			{href: "privacy", label: "Privacy policy", gcCom: true},
		}},
	}
	tplfunc.Add("help_nav", func(ctx context.Context, active string) template.HTML {
		var (
			dropdown = new(strings.Builder)
			list     = new(strings.Builder)
			w        func(context.Context, []x)
			e        = template.HTMLEscapeString
			gcCom    = Config(ctx).GoatcounterCom
		)
		w = func(ctx context.Context, l []x) {
			for _, ll := range l {
				if ll.gcCom && !gcCom {
					continue
				}

				if len(ll.items) > 0 {
					fmt.Fprintf(list, `<li><strong>%s</strong><ul>`, e(ll.label))
					fmt.Fprintf(dropdown, `<optgroup label="%s">`, e(ll.label))
					w(ctx, ll.items)
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
		w(ctx, links)
		dropdown.WriteString("</select>")
		list.WriteString("</ul>")
		return template.HTML(dropdown.String() + list.String())
	})
	tplfunc.Add("help_hdr", func(ctx context.Context, active string) template.HTML {
		if active == "404" {
			return "404: Not Found"
		}
		var w func(context.Context, []x) string
		w = func(ctx context.Context, l []x) string {
			for _, ll := range l {
				if ll.href == active {
					return strings.TrimRight(ll.label, "?")
				}
				if len(ll.items) > 0 {
					if r := w(ctx, ll.items); r != "" {
						return r
					}
				}
			}
			return ""
		}
		return template.HTML(w(ctx, links))
	})

	tplfunc.Add("dformat", func(t time.Time, withTime bool, u User) string {
		f := u.Settings.DateFormat
		if withTime {
			if u.Settings.TwentyFourHours {
				f += " 15:04"
			} else {
				f += " 03:04"
			}
		}

		return t.In(u.Settings.Timezone.Loc()).Format(f)
	})

	tplfunc.Add("path_id", func(p string) string {
		p = strings.ReplaceAll(strings.TrimLeft(p, "/"), "/", "-")
		if p == "" {
			return "dashboard"
		}
		return p
	})

	// Override defaults to take user settings in to account.
	tplfunc.Add("tformat", func(t time.Time, fmt string, u User) string {
		if fmt == "" {
			fmt = "2006-01-02"
		}
		return t.In(u.Settings.Timezone.Loc()).Format(fmt)
	})
	tplfunc.Add("nformat", func(n any, u User) string {
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

func HorizontalChart(ctx context.Context, stats HitStats, total int, link, paginate bool) template.HTML {
	if total == 0 || len(stats.Stats) == 0 {
		return template.HTML("<em>" + z18n.T(ctx, "dashboard/nothing-to-display|Nothing to display") + "</em>")
	}

	var (
		user      = MustGetUser(ctx)
		displayed int
		b         = new(strings.Builder)
	)
	b.WriteString(`<div class="rows">`)
	for _, s := range stats.Stats {
		displayed += s.Count

		var (
			p    = float64(s.Count) / float64(total) * 100
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

		name := ""
		if s.Name != "" {
			name = template.HTMLEscapeString(s.Name)
		} else {
			switch s.ID {
			case sizePhones:
				name = z18n.T(ctx, "label/size-phones|Phones")
			case sizeLargePhones:
				name = z18n.T(ctx, "label/size-largephones|Large phones, small tablets")
			case sizeTablets:
				name = z18n.T(ctx, "label/size-tablets|Tablets and small laptops")
			case sizeDesktop:
				name = z18n.T(ctx, "label/size-desktop|Computer monitors")
			case sizeDesktopHD:
				name = z18n.T(ctx, "label/size-desktophd|Computer monitors larger than HD")
			}
		}

		unknown := false
		if name == "" {
			name = z18n.T(ctx, "unknown|(unknown)")
			unknown = true
		}
		class := ""
		if unknown || (s.RefScheme != nil && string(*s.RefScheme) == *RefSchemeGenerated) {
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
		if link && !unknown {
			ref = fmt.Sprintf(`<a href="#" class="load-detail">`+
				`<span class="bar" style="width: %s"></span>`+
				`<span class="bar-c"><span class="cutoff">%s</span> %s</span></a>`, perc, ename, visit)
		} else {
			ref = fmt.Sprintf(`<span class="bar" style="width: %s"></span>`+
				`<span class="bar-c"><span class="cutoff">%s</span> %s</span>`, perc, ename, visit)
		}

		ncol := ""
		if !user.Settings.FewerNumbers {
			ncol = tplfunc.Number(s.Count, user.Settings.NumberFormat)
		}

		id := s.ID
		if id == "" {
			id = name
		}
		fmt.Fprintf(b, `
			<div class="%[1]s" data-key="%[2]s">
				<span class="col-count col-perc">%[3]s</span>
				<span class="col-name">%[4]s</span>
				<span class="col-count">%[5]s</span>
			</div>`,
			class, id, perc, ref, ncol)
	}
	b.WriteString(`</div>`)

	// Add pagination link.
	if paginate && stats.More {
		b.WriteString(`<a href="#" class="load-more">`)
		b.WriteString(z18n.T(ctx, "link/show-more|Show more"))
		b.WriteString("</a>")
		b.WriteString(`<a href="#" class="load-less">`)
		b.WriteString(z18n.T(ctx, "link/show-less|(show less)"))
		b.WriteString("</a>")
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
		Context context.Context
		Error   error
	}
	TplEmailExportDone struct {
		Context context.Context
		Site    Site
		User    User
		Export  Export
	}
	TplEmailImportDone struct {
		Context context.Context
		Site    Site
		Rows    int
		Errors  *errors.Group
	}
)

var tplE = ztpl.ExecuteBytes

func (t TplEmailWelcome) Render() ([]byte, error)       { return tplE("email_welcome.gotxt", t) }
func (t TplEmailForgotSite) Render() ([]byte, error)    { return tplE("email_forgot_site.gotxt", t) }
func (t TplEmailPasswordReset) Render() ([]byte, error) { return tplE("email_password_reset.gotxt", t) }
func (t TplEmailVerify) Render() ([]byte, error)        { return tplE("email_verify.gotxt", t) }
func (t TplEmailAddUser) Render() ([]byte, error)       { return tplE("email_adduser.gotxt", t) }
func (t TplEmailImportError) Render() ([]byte, error)   { return tplE("email_import_error.gotxt", t) }
func (t TplEmailExportDone) Render() ([]byte, error)    { return tplE("email_export_done.gotxt", t) }
func (t TplEmailImportDone) Render() ([]byte, error)    { return tplE("email_import_done.gotxt", t) }

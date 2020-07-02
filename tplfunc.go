// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"zgo.at/errors"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

func init() {
	zhttp.FuncMap["base32"] = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString
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

	zhttp.FuncMap["totp_barcode"] = func(email, s string) template.HTML {
		totpURI := fmt.Sprintf("otpauth://totp/GoatCounter:%s?secret=%s&issuer=GoatCounter", email, s)
		buf := bytes.NewBufferString("data:image/png;base64,")
		qrCode, err := qr.Encode(totpURI, qr.M, qr.Auto)
		if err != nil {
			zlog.Error(errors.Wrap(err, "encoding QR code"))
			return template.HTML("Error generating the QR code; this has been logged for investigation.")
		}

		qrCode, err = barcode.Scale(qrCode, 200, 200)
		if err != nil {
			zlog.Error(errors.Wrap(err, "scaling QR code"))
			return template.HTML("Error generating the QR code; this has been logged for investigation.")
		}

		e := base64.NewEncoder(base64.StdEncoding, buf)
		err = png.Encode(e, qrCode)
		if err != nil {
			zlog.Error(errors.Wrap(err, "encoding QR code as PNG"))
			return template.HTML("Error generating the QR code; this has been logged for investigation.")
		}

		return template.HTML(fmt.Sprintf(
			`<img alt="TOTP Secret Barcode" title="TOTP Secret Barcode" src="%s">`,
			buf.String()))
	}
}

func BarChart(ctx context.Context, stats []Stat, max int, daily bool) template.HTML {
	site := MustGetSite(ctx)
	now := Now().In(site.Settings.Timezone.Loc())
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

func HorizontalChart(ctx context.Context, stats Stats, total, pageSize int, link, paginate bool) template.HTML {
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
		perc := float32(s.CountUnique) / float32(total) * 100

		name := template.HTMLEscapeString(s.Name)
		if name == "" {
			name = "(unknown)"
		}
		class := ""
		if name == "(unknown)" || (s.RefScheme != nil && *s.RefScheme == *RefSchemeGenerated) {
			class = "generated"
		}
		visit := ""
		if !link && s.RefScheme != nil && *s.RefScheme == *RefSchemeHTTP {
			visit = fmt.Sprintf(
				`<sup class="go"><a rel="noopener" target="_blank" href="http://%s">visit</a></sup>`,
				name)
		}

		ref := name
		if link {
			ref = `<a href="#" class="load-detail">` + ref + `</a>`
		}
		b.WriteString(fmt.Sprintf(`
			<div class="%[1]s" data-name="%[2]s">
				<div class="bar" style="width: %.1[3]f%%"><small class="perc">%.1[3]f%%</small></div>
				<span class="col-count">%[5]s</span>
				<span class="col-name">%[4]s %[6]s</span>
			</div>`,
			class, name, perc, ref,
			zhttp.Tnformat(s.CountUnique, MustGetSite(ctx).Settings.NumberFormat),
			visit))
	}
	b.WriteString(`</div>`)

	// Add pagination link.
	if paginate && stats.More {
		b.WriteString(`<a href="#", class="load-more">Show more</a>`)
	}

	return template.HTML(b.String())
}

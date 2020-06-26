// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zfloat"
	"zgo.at/zvalidate"
)

const exportVersion = "1"

// ExportFile gets the filename used for an export.
func ExportFile(site *Site) string {
	return fmt.Sprintf("%s/goatcounter-export-%s.csv.gz", os.TempDir(), site.Code)
}

// Export all data to a CSV file.
func Export(ctx context.Context, fp *os.File, last int64) {
	site := MustGetSite(ctx)
	l := zlog.Module("export").Field("site", site.ID).Field("last", last)
	l.Print("export started")

	gzfp := gzip.NewWriter(fp)
	defer fp.Close() // No need to error-check; just for safety.
	defer gzfp.Close()

	c := csv.NewWriter(gzfp)
	c.Write([]string{exportVersion + "Path", "Title", "Event", "Bot", "Session",
		"FirstVisit", "Referrer", "Referrer scheme", "Browser", "Screen size",
		"Location", "Date"})

	var (
		err  error
		rows int
	)
	for {
		var hits Hits
		last, err = hits.List(ctx, 5000, last)
		if len(hits) == 0 {
			break
		}
		if err != nil {
			break
		}

		rows += len(hits)

		for _, hit := range hits {
			s := ""
			if hit.Session != nil {
				s = fmt.Sprintf("%d", *hit.Session)
			}

			rs := ""
			if hit.RefScheme != nil {
				rs = *hit.RefScheme
			}

			c.Write([]string{hit.Path, hit.Title, fmt.Sprintf("%t", hit.Event),
				fmt.Sprintf("%d", hit.Bot), s, fmt.Sprintf("%t", hit.FirstVisit),
				hit.Ref, rs, hit.Browser, zfloat.Join(hit.Size, ","),
				hit.Location, hit.CreatedAt.Format(time.RFC3339)})
		}

		c.Flush()
		err = c.Error()
		if err != nil {
			break
		}

		// Small amount of breathing space.
		if cfg.Prod {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if err != nil {
		l.Error(err)
		_ = gzfp.Close()
		_ = fp.Close()
		_ = os.Remove(fp.Name())
		return
	}

	err = gzfp.Close()
	if err != nil {
		l.Error(err)
		return
	}
	err = fp.Sync() // Ensure stat is correct.
	if err != nil {
		l.Error(err)
		return
	}

	stat, err := fp.Stat()
	size := "0"
	if err == nil {
		size = fmt.Sprintf("%.1f", float64(stat.Size())/1024/1024)
	}

	err = fp.Close()
	if err != nil {
		l.Error(err)
		return
	}

	f := ExportFile(site)
	err = os.Rename(fp.Name(), f)
	if err != nil {
		l.Error(err)
		return
	}

	hash, err := zcrypto.HashFile(f)
	if err != nil {
		l.Error(err)
		return
	}

	user := GetUser(ctx)
	err = blackmail.Send("GoatCounter export ready",
		blackmail.From("GoatCounter export", cfg.EmailFrom),
		blackmail.To(user.Email),
		blackmail.BodyMustText(EmailTemplate("email_export_done.gotxt", struct {
			Site   Site
			LastID int64
			Size   string
			Rows   int
			Hash   string
		}{*site, last, size, rows, hash})))
	if err != nil {
		l.Error(err)
	}
}

func importError(l zlog.Log, user User, report error) {
	if e, ok := report.(*errors.StackErr); ok {
		report = e.Unwrap()
	}

	err := blackmail.Send("GoatCounter import error",
		blackmail.From("GoatCounter import", cfg.EmailFrom),
		blackmail.To(user.Email),
		blackmail.BodyMustText(EmailTemplate("email_import_error.gotxt", struct {
			Error error
		}{report})))
	if err != nil {
		l.Error(err)
	}
}

// Import data from an export.
func Import(ctx context.Context, fp io.Reader, replace bool) {
	site := MustGetSite(ctx)
	user := GetUser(ctx)

	l := zlog.Module("import").Field("site", site.ID).Field("replace", replace)
	l.Print("import started")

	c := csv.NewReader(fp)
	header, err := c.Read()
	if err != nil {
		importError(l, *user, err)
		return
	}

	if len(header) == 0 || !strings.HasPrefix(header[0], exportVersion) {
		importError(l, *user, errors.Errorf(
			"wrong version of CSV database: %s (expected: %s)",
			header[0][:1], exportVersion))
		return
	}

	if replace {
		err := site.DeleteAll(ctx)
		if err != nil {
			importError(l, *user, err)
			l.Error(err)
			return
		}
	}

	var (
		sessions = make(map[string]int64)
		n        = 0
		errs     = errors.NewGroup(50)
	)
	for {
		row, err := c.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs.Append(err)
			continue
		}
		if len(row) != 12 {
			errs.Append(fmt.Errorf("wrong number of fields: %d (want: 12)", len(row)))
			continue
		}

		path, title, event, bot, session, firstVisit, ref, refScheme, browser,
			size, location, createdAt := row[0], row[1], row[2], row[3], row[4],
			row[5], row[6], row[7], row[8], row[9], row[10], row[11]

		hit := Hit{
			Site:     site.ID,
			Path:     path,
			Title:    title,
			Ref:      ref,
			Browser:  browser,
			Location: location, // TODO: validate from list?
		}

		v := zvalidate.New()
		v.Required("path", path)
		hit.Event = zdb.Bool(v.Boolean("event", event))
		hit.Bot = int(v.Integer("bot", bot))
		hit.FirstVisit = zdb.Bool(v.Boolean("firstVisit", firstVisit))
		hit.CreatedAt = v.Date("createdAt", createdAt, time.RFC3339)

		if refScheme != "" {
			v.Include("refScheme", refScheme, []string{*RefSchemeHTTP, *RefSchemeOther, *RefSchemeGenerated, *RefSchemeCampaign})
			hit.RefScheme = &refScheme
		}

		if size != "" {
			err = hit.Size.UnmarshalText([]byte(size))
			if err != nil {
				errs.Append(err)
				continue
			}
		}

		if v.HasErrors() {
			errs.Append(v)
			continue
		}

		// Map session IDs to new session IDs.
		s, ok := sessions[session]
		if !ok {
			err = zdb.MustGet(ctx).GetContext(ctx, &s, `select nextval('sessions_id_seq')`)
			if err != nil {
				errs.Append(err)
				continue
			}
			sessions[session] = s
		}
		hit.Session = &s

		Memstore.Append(hit)
		n++

		// Spread out the load a bit.
		if cfg.Prod && n%5000 == 0 {
			time.Sleep(10 * time.Second)
		}
	}

	l.Debugf("imported %d rows", n)
	if errs.Len() > 0 {
		l.Error(errs)
	}

	// Send email after 10s delay to make sure the cron task has finished
	// updating all the rows.
	time.Sleep(10 * time.Second)
	err = blackmail.Send("GoatCounter import ready",
		blackmail.From("GoatCounter import", cfg.EmailFrom),
		blackmail.To(user.Email),
		blackmail.BodyMustText(EmailTemplate("email_import_done.gotxt", struct {
			Site   Site
			Rows   int
			Errors *errors.Group
		}{*site, n, errs})))
	if err != nil {
		l.Error(err)
	}
}

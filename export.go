// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

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

type Export struct {
	ID     int64 `db:"export_id" json:"id,readonly"`
	SiteID int64 `db:"site_id" json:"site_id,readonly"`

	// The hit ID this export was started from.
	StartFromHitID int64 `db:"start_from_hit_id" json:"start_from_hit_id"`

	// Last hit ID that was exported; can be used as start_from_hit_id.
	LastHitID *int64 `db:"last_hit_id" json:"last_hit_id,readonly"`

	Path      string    `db:"path" json:"path,readonly"`
	CreatedAt time.Time `db:"created_at" json:"created_at,readonly"`

	FinishedAt *time.Time `db:"finished_at" json:"finished_at,readonly"`
	NumRows    *int       `db:"num_rows" json:"num_rows,readonly"`

	// File size in MB.
	Size *string `db:"size" json:"size,readonly"`

	// SHA256 hash.
	Hash *string `db:"hash" json:"hash,readonly"`

	// Any errors that may have occured.
	Error *string `db:"error" json:"error,readonly"`
}

func (e *Export) ByID(ctx context.Context, id int64) error {
	return errors.Wrapf(zdb.MustGet(ctx).GetContext(ctx, e,
		`/* Export.ByID */ select * from exports where export_id=$1 and site_id=$2`,
		id, MustGetSite(ctx).ID), "Export.ByID %d", id)
}

// Create a new export.
//
// Inserts a row in exports table and returns open file pointer to the
// destination file.
func (e *Export) Create(ctx context.Context, startFrom int64) (*os.File, error) {
	site := MustGetSite(ctx)

	e.SiteID = site.ID
	e.CreatedAt = Now()
	e.StartFromHitID = startFrom
	e.Path = fmt.Sprintf("%s%sgoatcounter-export-%s-%s-%d.csv.gz",
		os.TempDir(), string(os.PathSeparator), site.Code,
		e.CreatedAt.Format("20060102T150405Z"), startFrom)

	query := `insert into exports (site_id, path, created_at, start_from_hit_id) values ($1, $2, $3, $4)`
	args := []interface{}{e.SiteID, e.Path, e.CreatedAt.Format(zdb.Date), e.StartFromHitID}

	if cfg.PgSQL {
		err := zdb.MustGet(ctx).GetContext(ctx, &e.ID, query+` returning export_id`, args...)
		if err != nil {
			return nil, errors.Wrap(err, "Export.Create")
		}
	} else {
		res, err := zdb.MustGet(ctx).ExecContext(ctx, query, args...)
		if err != nil {
			return nil, errors.Wrap(err, "Export.Create")
		}
		e.ID, err = res.LastInsertId()
		if err != nil {
			return nil, errors.Wrap(err, "Export.Create")
		}
	}

	fp, err := os.Create(e.Path)
	return fp, errors.Wrap(err, "Export.Create")
}

// Export all data to a CSV file.
func (e *Export) Run(ctx context.Context, fp *os.File, mailUser bool) {
	l := zlog.Module("export").Field("id", e.ID)
	l.Print("export started")

	gzfp := gzip.NewWriter(fp)
	defer fp.Close() // No need to error-check; just for safety.
	defer gzfp.Close()

	c := csv.NewWriter(gzfp)
	c.Write([]string{exportVersion + "Path", "Title", "Event", "Bot", "Session",
		"FirstVisit", "Referrer", "Referrer scheme", "Browser", "Screen size",
		"Location", "Date"})

	var exportErr error
	e.LastHitID = &e.StartFromHitID
	var z int
	e.NumRows = &z
	for {
		var (
			hits Hits
			last int64
		)
		last, exportErr = hits.List(ctx, 5000, *e.LastHitID)
		e.LastHitID = &last
		if len(hits) == 0 {
			break
		}
		if exportErr != nil {
			break
		}

		*e.NumRows += len(hits)

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
		exportErr = c.Error()
		if exportErr != nil {
			break
		}

		// Small amount of breathing space.
		if cfg.Prod {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if exportErr != nil {
		l.Field("export", e).Error(exportErr)

		_, err := zdb.MustGet(ctx).ExecContext(ctx,
			`update exports set error=$1 where export_id=$2`,
			exportErr.Error(), e.ID)
		if err != nil {
			zlog.Error(err)
		}

		_ = gzfp.Close()
		_ = fp.Close()
		_ = os.Remove(fp.Name())
		return
	}

	err := gzfp.Close()
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
	e.Size = &size

	err = fp.Close()
	if err != nil {
		l.Error(err)
		return
	}

	hash, err := zcrypto.HashFile(e.Path)
	e.Hash = &hash
	if err != nil {
		l.Error(err)
		return
	}

	now := Now().Format(zdb.Date)
	_, err = zdb.MustGet(ctx).ExecContext(ctx, `update exports set
		finished_at=$1, num_rows=$2, size=$3, hash=$4, last_hit_id=$5
		where export_id=$6`,
		&now, e.NumRows, e.Size, e.Hash, e.LastHitID, e.ID)
	if err != nil {
		zlog.Error(err)
	}

	if mailUser {
		site := MustGetSite(ctx)
		user := GetUser(ctx)
		err = blackmail.Send("GoatCounter export ready",
			blackmail.From("GoatCounter export", cfg.EmailFrom),
			blackmail.To(user.Email),
			blackmail.BodyMustText(EmailTemplate("email_export_done.gotxt", struct {
				Site   Site
				Export Export
			}{*site, *e})))
		if err != nil {
			l.Error(err)
		}
	}
}

type Exports []Export

func (e *Exports) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, e, `/* Exports.List */
		select * from exports where site_id=$1 and created_at > `+interval(1),
		MustGetSite(ctx).ID), "Exports.List")
}

// Import data from an export.
func Import(ctx context.Context, fp io.Reader, replace, email bool) {
	site := MustGetSite(ctx)
	user := GetUser(ctx)
	db := zdb.MustGet(ctx)

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

	if !cfg.PgSQL {
		// Insert a row to ensure sessions is in sqlite_sequence; this won't be
		// added until the first sequences is accessed and is required for
		// Import to work.
		var s Session
		_, err := s.GetOrCreate(ctx, "", "", "")
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
			if cfg.PgSQL {
				err = db.GetContext(ctx, &s, `select nextval('sessions_id_seq')`)
			} else {
				err = zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
					_, err := tx.ExecContext(ctx, `update sqlite_sequence set seq=seq+1 where name='sessions'`)
					if err != nil {
						return err
					}
					return tx.GetContext(ctx, &s, `select seq from sqlite_sequence where name='sessions'`)
				})
			}
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

	if email {
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

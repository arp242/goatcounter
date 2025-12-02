package goatcounter

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
)

const ExportCSVVersion = "2"

// CreateCSV creates a new CSV export.
//
// Inserts a row in exports table and returns open file pointer to the
// destination file.
func (e *Export) CreateCSV(ctx context.Context, startFrom HitID) (*os.File, error) {
	site := MustGetSite(ctx)

	e.SiteID = site.ID
	e.CreatedAt = ztime.Now(ctx)
	e.StartFromHitID = startFrom
	e.Path = fmt.Sprintf("%s%sgoatcounter-export-%s-%s-%d.csv.gz",
		os.TempDir(), string(os.PathSeparator), site.Code,
		e.CreatedAt.Format("20060102T150405Z"), startFrom)

	var err error
	e.ID, err = zdb.InsertID[ExportID](ctx, "export_id",
		`insert into exports (site_id, path, created_at, start_from_hit_id) values (?, ?, ?, ?)`,
		e.SiteID, e.Path, e.CreatedAt, e.StartFromHitID)
	if err != nil {
		return nil, errors.Wrap(err, "Export.Create")
	}

	fp, err := os.Create(e.Path)
	return fp, errors.Wrap(err, "Export.Create")
}

// Export all data to a CSV file.
func (e *Export) RunCSV(ctx context.Context, fp *os.File, mailUser bool) {
	l := log.Module("export").With("id", e.ID)
	l.Info(ctx, "export started")

	gzfp := gzip.NewWriter(fp)
	defer fp.Close() // No need to error-check; just for safety.
	defer gzfp.Close()

	c := csv.NewWriter(gzfp)
	c.Write([]string{ExportCSVVersion + "Path", "Title", "Event", "UserAgent",
		"Browser", "System", "Session", "Bot", "Referrer", "Referrer scheme",
		"Screen size", "Location", "FirstVisit", "Date"})

	var exportErr error
	e.LastHitID = &e.StartFromHitID
	var z int
	e.NumRows = &z
	for {
		var (
			hits ExportCSVRows
			last HitID
		)
		last, exportErr = hits.Export(ctx, 5000, *e.LastHitID)
		e.LastHitID = &last
		if len(hits) == 0 {
			break
		}
		if exportErr != nil {
			break
		}

		*e.NumRows += len(hits)

		for _, hit := range hits {
			c.Write([]string{hit.Path, hit.Title, hit.Event, hit.UserAgent,
				hit.Browser, hit.System, hit.Session.String(), "0", hit.Ref,
				hit.RefScheme, hit.Size, hit.Location, hit.FirstVisit,
				hit.CreatedAt})
		}

		c.Flush()
		exportErr = c.Error()
		if exportErr != nil {
			break
		}

		// Small amount of breathing space.
		if !Config(ctx).Dev {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if exportErr != nil {
		l.Error(ctx, exportErr, "export", e)
		err := zdb.Exec(ctx,
			`update exports set error=$1 where export_id=$2`,
			exportErr.Error(), e.ID)
		if err != nil {
			log.Error(ctx, err)
		}

		_ = gzfp.Close()
		_ = fp.Close()
		_ = os.Remove(fp.Name())
		return
	}

	err := gzfp.Close()
	if err != nil {
		l.Error(ctx, err)
		return
	}
	err = fp.Sync() // Ensure stat is correct.
	if err != nil {
		l.Error(ctx, err)
		return
	}

	stat, err := fp.Stat()
	size := "0"
	if err == nil {
		size = fmt.Sprintf("%.1f", float64(stat.Size())/1024/1024)
		if size == "0.0" {
			size = "0.1"
		}
	}
	e.Size = &size

	err = fp.Close()
	if err != nil {
		l.Error(ctx, err)
		return
	}

	hash, err := zcrypto.HashFile(e.Path)
	e.Hash = &hash
	if err != nil {
		l.Error(ctx, err)
		return
	}

	now := ztime.Now(ctx)
	err = zdb.Exec(ctx, `update exports set
		finished_at=$1, num_rows=$2, size=$3, hash=$4, last_hit_id=$5
		where export_id=$6`,
		&now, e.NumRows, e.Size, e.Hash, e.LastHitID, e.ID)
	if err != nil {
		log.Error(ctx, err)
	}

	if mailUser {
		site := MustGetSite(ctx)
		user := GetUser(ctx)
		err = blackmail.Send("GoatCounter export ready",
			blackmail.From("GoatCounter export", Config(ctx).EmailFrom),
			blackmail.To(user.Email),
			blackmail.HeadersAutoreply(),
			blackmail.BodyMustText(TplEmailExportDone{ctx, *site, *user, *e}.Render))
		if err != nil {
			l.Error(ctx, err)
		}
	}
}

// ImportCSV imports data from a CSV export.
//
// The persist() callback will be called for every hit; you usually want to
// collect a bunch of them and then persist them.
//
// After everything is done, this will be called once more with an empty hit and
// final set to true, to persist the last batch.
func ImportCSV(
	ctx context.Context, fp io.Reader, replace, email bool,
	persist func(Hit, bool),
) (*time.Time, error) {
	site := MustGetSite(ctx)

	l := log.Module("import").With("site", site.ID, "replace", replace)
	l.Info(ctx, "import started")

	c := csv.NewReader(fp)
	header, err := c.Read()
	if err != nil {
		return nil, errors.Wrap(err, "goatcounter.Import")
	}

	if len(header) == 0 || !strings.HasPrefix(header[0], ExportCSVVersion) {
		return nil, errors.Errorf(
			"goatcounter.Import: wrong version of CSV database: %s (expected: %s)",
			header[0][:1], ExportCSVVersion)
	}

	if replace {
		err := site.DeleteAll(ctx)
		if err != nil {
			l.Error(ctx, err)
			return nil, errors.Wrap(err, "goatcounter.Import")
		}
	}

	var (
		sessions   = make(map[zint.Uint128]zint.Uint128)
		n          = 0
		errs       = errors.NewGroup(50)
		firstHitAt = site.FirstHitAt
	)
	for {
		line, err := c.Read()
		if err == io.EOF {
			break
		}
		if errs.Append(err) {
			continue
		}

		var row ExportCSVRow
		err = row.Read(line)
		if errs.Append(err) {
			continue
		}

		hit, err := row.Hit(ctx, site.ID)
		if errs.Append(err) {
			continue
		}
		if hit.CreatedAt.Before(firstHitAt) {
			firstHitAt = hit.CreatedAt
		}

		// Map session IDs to new session IDs.
		s, ok := sessions[row.Session]
		if !ok {
			sessions[row.Session] = Memstore.SessionID()
			s = sessions[row.Session]
		}
		hit.Session = s

		persist(hit, false)
		n++
	}
	persist(Hit{}, true)

	l.Infof(ctx, "imported %d rows", n)
	if errs.Len() > 0 {
		l.Error(ctx, errs)
	}

	if email {
		// Send email after 10s delay to make sure the cron task has finished
		// updating all the rows.
		time.Sleep(10 * time.Second)
		err = blackmail.Send("GoatCounter import ready",
			blackmail.From("GoatCounter import", Config(ctx).EmailFrom),
			blackmail.To(GetUser(ctx).Email),
			blackmail.BodyMustText(TplEmailImportDone{ctx, *site, n, errs}.Render))
		if err != nil {
			l.Error(ctx, err)
		}
	}

	if firstHitAt.Equal(site.FirstHitAt) {
		return nil, nil
	}
	return &firstHitAt, nil
}

// TODO: would be nice to have generic csv marshal/unmarshaler, so you can do:
//
//    Path string `csv:"1"`
//
// Or something, or perhaps even get by header:
//
//    Path string `csv:"path"`
//
// Looks like there's some existing stuff for that already:
//
// https://github.com/gocarina/gocsv
// https://github.com/jszwec/csvutil

type ExportCSVRow struct { // Fields in order!
	ID     HitID  `db:"hit_id"`
	SiteID SiteID `db:"site_id"`

	Path  string `db:"path"`
	Title string `db:"title"`
	Event string `db:"event"`

	UserAgent string `db:"ua"`
	Browser   string `db:"browser"`
	System    string `db:"system"`

	Session    zint.Uint128 `db:"session"`
	Bot        string       `db:"bot"`
	Ref        string       `db:"ref"`
	RefScheme  string       `db:"ref_s"`
	Size       string       `db:"size"`
	Location   string       `db:"loc"`
	FirstVisit string       `db:"first"`
	CreatedAt  string       `db:"created_at"`
}

func (row *ExportCSVRow) Read(line []string) error {
	const offset = 2 // Ignore first n fields

	values := reflect.ValueOf(row).Elem()
	if len(line) != values.NumField()-offset {
		return fmt.Errorf("wrong number of fields: %d (want: %d)", len(line), values.NumField()-offset)
	}

	for i := offset; i <= len(line)+1; i++ {
		f := values.Field(i)
		v := line[i-offset]

		switch f.Kind() {
		default:
		case reflect.Array:
			zi, _ := zint.ParseUint128(v, 16)
			f.Set(reflect.ValueOf(zi))
		case reflect.Ptr:
			f.Set(reflect.New(f.Type().Elem()))
			f.Elem().SetString(v)
		case reflect.String:
			f.SetString(v)
		}
	}

	return nil
}

func (row ExportCSVRow) Hit(ctx context.Context, siteID SiteID) (Hit, error) {
	hit := Hit{
		Site:            siteID,
		Path:            row.Path,
		Title:           row.Title,
		Ref:             row.Ref,
		UserAgentHeader: row.UserAgent,
		Location:        row.Location, // TODO: validate from list?
	}

	v := NewValidate(ctx)
	v.Required("path", row.Path)
	hit.Event = zbool.Bool(v.Boolean("event", row.Event))
	hit.Bot = int(v.Integer("bot", row.Bot))
	hit.FirstVisit = zbool.Bool(v.Boolean("firstVisit", row.FirstVisit))
	hit.CreatedAt = v.Date("createdAt", row.CreatedAt, time.RFC3339)

	if row.RefScheme != "" {
		v.Include("refScheme", row.RefScheme,
			[]string{*RefSchemeHTTP, *RefSchemeOther, *RefSchemeGenerated, *RefSchemeCampaign})
		if row.RefScheme != "" {
			hit.RefScheme = &row.RefScheme
		}
	}

	if row.Size != "" {
		err := hit.Size.UnmarshalText([]byte(row.Size))
		return hit, err
	}

	return hit, v.ErrorOrNil()
}

type ExportCSVRows []ExportCSVRow

// Export all hits for a site, including bot requests.
func (h *ExportCSVRows) Export(ctx context.Context, limit, paginate HitID) (HitID, error) {
	if limit == 0 || limit > 5000 {
		limit = 5000
	}

	err := zdb.Select(ctx, h, `
		select
			hits.hit_id,
			hits.site_id,

			paths.path,
			paths.title,
			paths.event,

			coalesce(browsers.name || ' ' || browsers.version, '') as browser,
			coalesce(systems.name  || ' ' || systems.version, '')  as system,

			hits.session,
			coalesce(refs.ref, '')           as ref,
			coalesce(refs.ref_scheme, '')    as ref_s,
			coalesce(hits.width, 0) ||',0,1' as size,
			coalesce(hits.location, '')      as loc,
			hits.first_visit                 as first,
			hits.created_at
		from hits
		join paths         using (path_id)
		left join refs     using (ref_id)
		left join browsers using (browser_id)
		left join systems  using (system_id)
		where hits.site_id=$1 and hit_id>$2
		order by hit_id asc
		limit $3`,
		MustGetSite(ctx).ID, paginate, limit)

	last := paginate
	if len(*h) > 0 {
		hh := *h
		last = hh[len(hh)-1].ID
	}

	return last, errors.Wrap(err, "Hits.List")
}

package goatcounter

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/guru"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zfilepath"
	"zgo.at/zstd/zslice"
	"zgo.at/zstd/ztime"
)

const ExportJSONVersion = "1.0"

type (
	ExportInfo struct {
		ExportVersion      string    `json:"export_version"`
		GoatcounterVersion string    `json:"goatcounter_version"`
		CreatedFor         string    `json:"created_for"`
		CreatedBy          string    `json:"created_by"`
		CreatedAt          time.Time `json:"created_at"`
	}
	ExportPath struct {
		ID    PathID     `db:"path_id" json:"id"`
		Path  string     `db:"path" json:"path"`
		Title string     `db:"title" json:"title"`
		Event zbool.Bool `db:"event" json:"event,omitempty"`
	}
	ExportLanguage struct {
		ISO6393 string `db:"iso_639_3" json:"iso_639_3"`
		Name    string `db:"name" json:"name"`
	}
	ExportBrowserStat struct {
		Day       string    `db:"day" json:"day"`
		PathID    PathID    `db:"path_id" json:"path_id"`
		BrowserID BrowserID `db:"browser_id" json:"browser_id"`
		Count     int       `db:"count" json:"count"`
	}
	ExportSystemStat struct {
		Day      string   `db:"day" json:"day"`
		PathID   PathID   `db:"path_id" json:"path_id"`
		SystemID SystemID `db:"system_id" json:"system_id"`
		Count    int      `db:"count" json:"count"`
	}
	ExportLocationStat struct {
		Day      string `db:"day" json:"day"`
		PathID   PathID `db:"path_id" json:"path_id"`
		Location string `db:"location" json:"location"`
		Count    int    `db:"count" json:"count"`
	}
	ExportSizeStat struct {
		Day    string `db:"day" json:"day"`
		PathID PathID `db:"path_id" json:"path_id"`
		Width  int    `db:"width" json:"width"`
		Count  int    `db:"count" json:"count"`
	}
	ExportLanguageStat struct {
		Day      string `db:"day" json:"day"`
		PathID   PathID `db:"path_id" json:"path_id"`
		Language string `db:"language" json:"language"`
		Count    int    `db:"count" json:"count"`
	}
	ExportCampaignStat struct {
		Day        string     `db:"day" json:"day"`
		PathID     PathID     `db:"path_id" json:"path_id"`
		CampaignID CampaignID `db:"campaign_id" json:"campaign_id"`
		Ref        string     `db:"ref" json:"ref"`
		Count      int        `db:"count" json:"count"`
	}
	ExportHitStat struct {
		Hour   string `db:"hour" json:"hour"`
		PathID PathID `db:"path_id" json:"path_id"`
		RefID  RefID  `db:"ref_id" json:"ref_id"`
		Count  int    `db:"total" json:"count"`
	}
)

// CreateJSON creates a new JSON export.
//
// Inserts a row in exports table and returns open file pointer to the
// destination file.
func (e *Export) CreateJSON(ctx context.Context, periodStart time.Time) (*os.File, error) {
	site := MustGetSite(ctx)

	e.SiteID = site.ID
	e.CreatedAt = ztime.Now(ctx)
	e.Format = "json"
	if !periodStart.IsZero() {
		e.StartFromDay = new(periodStart.Truncate(time.Hour * 24))
	}
	e.Path = fmt.Sprintf("%s%sgoatcounter-export-%s-%s.zip",
		os.TempDir(), string(os.PathSeparator), site.Code,
		e.CreatedAt.Format("20060102T150405Z"))

	err := zdb.Insert(ctx, e)
	if err != nil {
		return nil, errors.Wrap(err, "Export.CreateJSON")
	}

	fp, err := os.Create(e.Path)
	return fp, errors.Wrap(err, "Export.CreateJSON")
}

func (e *Export) RunJSON(ctx context.Context, fp *os.File, mailUser bool) {
	l := log.Module("export").With("id", e.ID)
	l.Info(ctx, "JSON export started")

	siteID := MustGetSite(ctx).ID

	z := zip.NewWriter(fp)
	defer func() {
		z.Close()
		fp.Close()
	}()

	var (
		whereDay, whereHour string
		params              = map[string]any{"site_id": siteID}
	)
	if e.StartFromDay != nil {
		params["start_from_day"] = e.StartFromDay.Format("2006-01-02")
		params["start_from_hour"] = e.StartFromDay.Format("2006-01-02 15:04:05")
		whereDay, whereHour = "and day >= :start_from_day", "and hour >= :start_from_hour"
	}
	tables := []struct {
		p string
		f func(w io.Writer) error
	}{
		{"paths", func(w io.Writer) error {
			return queryToJSON[ExportPath](ctx, w,
				`select path_id, path, title, event from paths where site_id=:site_id`, params)
		}},
		{"refs", func(w io.Writer) error {
			return queryToJSON[Ref](ctx, w, `
				select ref_id, ref, ref_scheme from refs
				where ref_id in (select ref_id from ref_counts where site_id=:site_id group by ref_id)
				order by ref_id asc`, params)
		}},
		{"browsers", func(w io.Writer) error {
			return queryToJSON[Browser](ctx, w, `select * from browsers`)
		}},
		{"systems", func(w io.Writer) error {
			return queryToJSON[System](ctx, w, `select * from systems`)
		}},
		{"locations", func(w io.Writer) error {
			return queryToJSON[Location](ctx, w, `select location_id, country, region, country_name, region_name from locations`)
		}},
		{"languages", func(w io.Writer) error {
			return queryToJSON[ExportLanguage](ctx, w, `select * from languages`)
		}},
		{"browser_stats", func(w io.Writer) error {
			return queryToJSON[ExportBrowserStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, browser_id, count
				from browser_stats
				where site_id=:site_id `+whereDay+` order by day asc`, params)
		}},
		{"system_stats", func(w io.Writer) error {
			return queryToJSON[ExportSystemStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, system_id, count
				from system_stats
				where site_id=:site_id `+whereDay+` order by day asc`, params)
		}},
		{"location_stats", func(w io.Writer) error {
			return queryToJSON[ExportLocationStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, location, count
				from location_stats
				where site_id=:site_id `+whereDay+` order by day asc`, params)
		}},
		{"size_stats", func(w io.Writer) error {
			return queryToJSON[ExportSizeStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, width, count
				from size_stats
				where site_id=:site_id `+whereDay+` order by day asc`, params)
		}},
		{"language_stats", func(w io.Writer) error {
			return queryToJSON[ExportLanguageStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, language, count
				from language_stats
				where site_id=:site_id `+whereDay+` order by day asc`, params)
		}},
		//{"campaign_stats", func(w io.Writer) error {
		//	return queryToJSON[ExportCampaignStat](ctx, w, `
		//		select substr(cast(day as text), 0, 11) as day, path_id, campaign_id, ref, count
		//		from campaign_stats
		//		where site_id=:site_id `+whereDay+` order by day asc`, params)
		//}},

		// Export ref_counts as hit_stats. hit_counts is identical, just without
		// the ref_id dimension. Don't really need to include both in the
		// export.
		{"hit_stats", func(w io.Writer) error {
			return queryToJSON[ExportHitStat](ctx, w, `
				select hour, path_id, ref_id, total
				from ref_counts
				where site_id=:site_id `+whereHour+` order by hour asc`, params)
		}},
	}

	dir := filepath.Base(e.Path)
	dir, _ = zfilepath.SplitExt(dir)

	{ // Write info
		w, err := z.Create(filepath.Join(dir, "info.json"))
		if err != nil {
			l.Error(ctx, "err", err)
			return
		}
		u := MustGetUser(ctx)
		j, _ := json.MarshalIndent(ExportInfo{
			ExportVersion:      ExportJSONVersion,
			GoatcounterVersion: Version,
			CreatedFor:         GetSite(ctx).Display(ctx),
			CreatedBy:          fmt.Sprintf("user_id=%d <%s>", u.ID, u.Email),
			CreatedAt:          ztime.Now(ctx).Truncate(time.Second),
		}, "", "  ")
		if err != nil {
			l.Error(ctx, "err", err)
			return
		}
		_, err = w.Write(append(j, '\n'))
		if err != nil {
			l.Error(ctx, "err", err)
			return
		}
	}

	err := zdb.TX(ctx, func(ctx context.Context) error {
		for _, t := range tables {
			w, err := z.Create(filepath.Join(dir, t.p+".jsonl"))
			if err != nil {
				return fmt.Errorf("%q: %s", t.p, err)
			}
			err = t.f(w)
			if err != nil {
				return fmt.Errorf("%q: %s", t.p, err)
			}
		}
		return nil
	})
	if err != nil {
		l.Error(ctx, "err", err)
		return
	}

	err = z.Close()
	if err != nil {
		l.Error(ctx, "err", err)
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
		l.Error(ctx, "err", err)
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
		l.Error(ctx, err)
	}

	if mailUser {
		site := MustGetSite(ctx)
		user := GetUser(ctx)
		err = blackmail.Get(ctx).Send("GoatCounter export ready",
			blackmail.From("GoatCounter export", Config(ctx).EmailFrom),
			blackmail.To(user.Email),
			blackmail.HeadersAutoreply(),
			blackmail.BodyMustText(TplEmailExportDone{ctx, *site, *user, *e}.Render))
		if err != nil {
			l.Error(ctx, err)
		}
	}
}

func queryToJSON[T any](ctx context.Context, w io.Writer, q string, params ...any) error {
	rows, err := zdb.Query(ctx, "/* ExportJSON */\n"+q, params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if rows.Err() != nil {
			return rows.Err()
		}
		var t T
		err := rows.StructScan(&t)
		if err != nil {
			return err
		}
		j, err := json.Marshal(t)
		if err != nil {
			return err
		}
		_, err = w.Write(j)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{'\n'})
		if err != nil {
			return err
		}
	}
	return nil
}

func ImportJSON(ctx context.Context, tmp string, replace, email bool) (*time.Time, error) {
	site := MustGetSite(ctx)

	l := log.Module("import").With("site", site.ID, "replace", replace)
	l.Info(ctx, "import started")

	if replace {
		err := site.DeleteAll(ctx)
		if err != nil {
			l.Error(ctx, err)
			return nil, errors.Wrap(err, "ImportJSON")
		}
	}

	zipf, err := zip.OpenReader(tmp)
	if err != nil {
		return nil, errors.Wrap(err, "ImportJSON")
	}

	// We allow any directory name to be present in the .zip (or completely
	// omitting it) and we want to import data before stats, so we need to split
	// them out.
	var (
		dataFiles = make([]string, 0, 12)
		statFiles = make([]string, 0, 12)
		have      = make(map[string]struct{})
	)
	{
		for _, f := range zipf.File {
			name := filepath.Base(f.Name)
			if _, ok := have[name]; ok {
				return nil, fmt.Errorf("ImportJSON: duplicate file: %q", name)
			}
			have[name] = struct{}{}
			switch name {
			case "paths.jsonl", "refs.jsonl", "browsers.jsonl", "systems.jsonl", "locations.jsonl", "languages.jsonl":
				dataFiles = append(dataFiles, f.Name)
			case "browser_stats.jsonl", "system_stats.jsonl", "location_stats.jsonl", "size_stats.jsonl",
				"language_stats.jsonl", "campaign_stats.jsonl", "hit_stats.jsonl":
				statFiles = append(statFiles, f.Name)
			case "info.json": // Validate version.
				fp, err := f.Open()
				if err != nil {
					return nil, guru.Errorf(400, "reading info.json: %s", err)
				}
				defer fp.Close()

				var info ExportInfo
				err = json.NewDecoder(fp).Decode(&info)
				if err != nil {
					return nil, guru.Errorf(400, "reading info.json: %s", err)
				}
				// Probably want to expand this version once we add more
				// versions, but for now we only have "1.0".
				if info.ExportVersion > ExportJSONVersion {
					return nil, guru.Errorf(400,
						"unknown export version %q; this version of GoatCounter (%s) only supports up to version %q",
						info.ExportVersion, Version, ExportJSONVersion)
				}
			}
		}
		want := []string{"paths.jsonl", "refs.jsonl", "browsers.jsonl", "systems.jsonl",
			"locations.jsonl", "languages.jsonl", "browser_stats.jsonl", "system_stats.jsonl",
			"location_stats.jsonl", "size_stats.jsonl", "language_stats.jsonl", "hit_stats.jsonl"}
		if m := zslice.Difference(want, slices.Collect(maps.Keys(have))); len(m) > 0 {
			return nil, guru.Errorf(400, "files are missing from the export .zip file: %s", m)
		}
	}

	imp := importerJSON{
		pathIDs:     make(map[PathID]PathID),
		refIDs:      make(map[RefID]RefID),
		browserIDs:  make(map[BrowserID]BrowserID),
		systemIDs:   make(map[SystemID]SystemID),
		campaignIDs: make(map[CampaignID]CampaignID),
	}
	err = zdb.TX(ctx, func(ctx context.Context) error {
		err := imp.data(ctx, zipf, dataFiles)
		if err != nil {
			return err
		}
		return imp.stats(ctx, zipf, statFiles)
	})
	if err != nil {
		return nil, errors.Wrap(err, "ImportJSON")
	}

	if email {
		err = blackmail.Get(ctx).Send("GoatCounter import ready",
			blackmail.From("GoatCounter import", Config(ctx).EmailFrom),
			blackmail.To(GetUser(ctx).Email),
			blackmail.BodyMustText(TplEmailImportDone{ctx, *site, imp.count, new(errors.Group)}.Render))
		if err != nil {
			l.Error(ctx, err)
		}
	}

	if imp.firstHitAt.Equal(site.FirstHitAt) {
		return nil, nil
	}
	return &imp.firstHitAt, nil
}

type importerJSON struct {
	pathIDs     map[PathID]PathID
	refIDs      map[RefID]RefID
	browserIDs  map[BrowserID]BrowserID
	systemIDs   map[SystemID]SystemID
	campaignIDs map[CampaignID]CampaignID
	firstHitAt  time.Time
	count       int
}

func (imp *importerJSON) data(ctx context.Context, zipf *zip.ReadCloser, dataFiles []string) error {
	var (
		site   = MustGetSite(ctx)
		tables = map[string]struct {
			table, idcol, conflict string
			cols                   []string
			values                 func(*zdb.BulkInsert, []byte) (int64, error)
		}{
			"paths.jsonl": {
				table:    "paths",
				idcol:    "path_id",
				conflict: "site_id, lower(path)",
				cols:     []string{"site_id", "path", "title", "event"},
				values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
					var v Path
					err := json.Unmarshal(line, &v)
					if err != nil {
						return 0, err
					}
					b.Values(site.ID, v.Path, v.Title, v.Event)
					return int64(v.ID), nil
				},
			},
			"refs.jsonl": {
				table:    "refs",
				idcol:    "ref_id",
				conflict: "lower(ref), ref_scheme",
				cols:     []string{"ref", "ref_scheme"},
				values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
					var v Ref
					err := json.Unmarshal(line, &v)
					if err != nil {
						return 0, err
					}
					b.Values(v.Ref, v.RefScheme)
					return int64(v.ID), nil
				},
			},
			"browsers.jsonl": {
				table:    "browsers",
				idcol:    "browser_id",
				conflict: "name, version",
				cols:     []string{"name", "version"},
				values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
					var v Browser
					err := json.Unmarshal(line, &v)
					if err != nil {
						return 0, err
					}
					b.Values(v.Name, v.Version)
					return int64(v.ID), nil
				},
			},
			"systems.jsonl": {
				table:    "systems",
				idcol:    "system_id",
				conflict: "name, version",
				cols:     []string{"name", "version"},
				values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
					var v System
					err := json.Unmarshal(line, &v)
					if err != nil {
						return 0, err
					}
					b.Values(v.Name, v.Version)
					return int64(v.ID), nil
				},
			},
			"locations.jsonl": {
				table:    "locations",
				idcol:    "location_id",
				conflict: "iso_3166_2",
				cols:     []string{"country", "region", "country_name", "region_name"},
				values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
					var v Location
					err := json.Unmarshal(line, &v)
					if err != nil {
						return 0, err
					}
					b.Values(v.Country, v.Region, v.CountryName, v.RegionName)
					return int64(v.ID), nil
				},
			},
			"languages.jsonl": {
				table: "languages",
				idcol: "",
				cols:  []string{"iso_639_3", "name"},
				values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
					var v ExportLanguage
					err := json.Unmarshal(line, &v)
					if err != nil {
						return 0, err
					}
					b.Values(v.ISO6393, v.Name)
					return 0, nil
				},
			},
			// "campaigns.jsonl": {
			// 	table:    "campaigns",
			// 	idcol:    "campaign_id",
			// 	conflict: "",
			// 	cols:     []string{"name"},
			// 	values: func(b *zdb.BulkInsert, line []byte) (int64, error) {
			// 		var v Campaign
			// 		err := json.Unmarshal(line, &v)
			// 		if err != nil {
			// 			return 0, err
			// 		}
			// 		b.Values(v.Name)
			// 		return int64(v.ID), nil
			// 	},
			// },
		}
	)

	for _, file := range dataFiles {
		rc, err := zipf.Open(file)
		if err != nil {
			return fmt.Errorf("reading %q: %w", file, err)
		}
		t := tables[filepath.Base(file)]

		b, err := zdb.NewBulkInsert(ctx, t.table, t.cols)
		if err != nil {
			return errors.Wrapf(err, "%q", t.table)
		}
		var (
			bp     = &b
			scan   = bufio.NewScanner(rc)
			i      int
			oldIDs = make([]int64, 0, 16)
		)
		if t.idcol != "" {
			b.Returning(t.idcol)
			// On ignore it won't return the ID column, so do a "fake update".
			//
			// TODO: on SQLite at least this will make the autoincrement
			// skip an ID (has nothing to do with idcol being used here;
			// happens on any column). See if we can avoid that.
			b.OnConflict(`on conflict do update set ` + t.idcol + `=` + t.table + `.` + t.idcol)
			if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
				// PostgreSQL needs column list.
				b.OnConflict(`on conflict (` + t.conflict + `) do update set ` + t.idcol + `=` + t.table + `.` + t.idcol)
			}
		} else { // Just for languages.
			b.OnConflict(`on conflict do nothing`)
		}
		for scan.Scan() {
			i++
			line := scan.Bytes()
			id, err := t.values(bp, line)
			if err != nil {
				return errors.Wrapf(err, "%q: line %d", t.table, i)
			}
			oldIDs = append(oldIDs, id)
		}
		if scan.Err() != nil {
			return fmt.Errorf("reading %q: %s", file, scan.Err())
		}
		err = b.Finish()
		if err != nil {
			return errors.Wrapf(err, "inserting %q", t.table)
		}

		for i, id := range b.Returned() {
			if len(id) != 1 {
				return fmt.Errorf("incorrect returned columns for %s: %#v", t.table, id)
			}
			n, ok := id[0].(int64)
			if !ok {
				return fmt.Errorf("invalid return type for %s: %#v", t.table, id)
			}
			switch t.table {
			case "paths":
				imp.pathIDs[PathID(oldIDs[i])] = PathID(n)
			case "refs":
				imp.refIDs[RefID(oldIDs[i])] = RefID(n)
			case "browsers":
				imp.browserIDs[BrowserID(oldIDs[i])] = BrowserID(n)
			case "systems":
				imp.systemIDs[SystemID(oldIDs[i])] = SystemID(n)
			case "campaigns":
				imp.campaignIDs[CampaignID(oldIDs[i])] = CampaignID(n)
			}
		}
	}
	return nil
}

func (imp *importerJSON) stats(ctx context.Context, zipf *zip.ReadCloser, statFiles []string) error {
	var (
		site         = MustGetSite(ctx)
		hitCounts    = make(map[PathID]ExportHitStat)
		lastHour     string
		hitBulk, err = Tables.HitCounts.Bulk(ctx)
	)
	if err != nil {
		return err
	}
	tables := map[string]struct {
		tbl    tbl
		values func(*zdb.BulkInsert, []byte) error
	}{
		"browser_stats.jsonl": {
			tbl: Tables.BrowserStats,
			values: func(b *zdb.BulkInsert, line []byte) error {
				var v ExportBrowserStat
				err := json.Unmarshal(line, &v)
				if err != nil {
					return err
				}
				b.Values(site.ID, imp.pathIDs[v.PathID], v.Day, imp.browserIDs[v.BrowserID], v.Count)
				return nil
			},
		},
		"system_stats.jsonl": {
			tbl: Tables.SystemStats,
			values: func(b *zdb.BulkInsert, line []byte) error {
				var v ExportSystemStat
				err := json.Unmarshal(line, &v)
				if err != nil {
					return err
				}
				b.Values(site.ID, imp.pathIDs[v.PathID], v.Day, imp.systemIDs[v.SystemID], v.Count)
				return nil
			},
		},
		"location_stats.jsonl": {
			tbl: Tables.LocationStats,
			values: func(b *zdb.BulkInsert, line []byte) error {
				var v ExportLocationStat
				err := json.Unmarshal(line, &v)
				if err != nil {
					return err
				}
				b.Values(site.ID, imp.pathIDs[v.PathID], v.Day, v.Location, v.Count)
				return nil
			},
		},
		"size_stats.jsonl": {
			tbl: Tables.SizeStats,
			values: func(b *zdb.BulkInsert, line []byte) error {
				var v ExportSizeStat
				err := json.Unmarshal(line, &v)
				if err != nil {
					return err
				}
				b.Values(site.ID, imp.pathIDs[v.PathID], v.Day, v.Width, v.Count)
				return nil
			},
		},
		"language_stats.jsonl": {
			tbl: Tables.LanguageStats,
			values: func(b *zdb.BulkInsert, line []byte) error {
				var v ExportLanguageStat
				err := json.Unmarshal(line, &v)
				if err != nil {
					return err
				}
				b.Values(site.ID, imp.pathIDs[v.PathID], v.Day, v.Language, v.Count)
				return nil
			},
		},
		//"campaign_stats.jsonl": {
		//	tbl: Tables.CampaignStats,
		//	values: func(b *zdb.BulkInsert, line []byte) error {
		//		var v ExportCampaignStat
		//		err := json.Unmarshal(line, &v)
		//		if err != nil {
		//			return err
		//		}
		//		b.Values(site.ID, imp.pathIDs[v.PathID], v.Day, imp.campaignIDs[v.CampaignID], v.Ref, v.Count)
		//		return nil
		//	},
		//},
		"hit_stats.jsonl": {
			tbl: Tables.RefCounts,
			values: func(b *zdb.BulkInsert, line []byte) error {
				var v ExportHitStat
				err := json.Unmarshal(line, &v)
				if err != nil {
					return err
				}

				if v.Hour != lastHour && len(hitCounts) > 0 {
					for _, hc := range hitCounts {
						hitBulk.Values(site.ID, imp.pathIDs[hc.PathID], hc.Hour[:10]+" "+hc.Hour[11:19], hc.Count)
					}
					clear(hitCounts)
				}

				lastHour = v.Hour
				if ex, ok := hitCounts[v.PathID]; ok {
					ex.Count += v.Count
					hitCounts[v.PathID] = ex
				} else {
					hitCounts[v.PathID] = v
				}
				if imp.firstHitAt.IsZero() {
					imp.firstHitAt, err = time.Parse(time.RFC3339, v.Hour)
					if err != nil {
						return err
					}
				}

				imp.count += v.Count
				b.Values(site.ID, imp.pathIDs[v.PathID], v.Hour[:10]+" "+v.Hour[11:19], imp.refIDs[v.RefID], v.Count)
				return nil
			},
		},
	}

	for _, file := range statFiles {
		rc, err := zipf.Open(file)
		if err != nil {
			return fmt.Errorf("reading %q: %s", file, err)
		}
		t, ok := tables[filepath.Base(file)]
		if !ok {
			return fmt.Errorf("no table for %q", filepath.Base(file))
		}

		b, err := t.tbl.Bulk(ctx)
		if err != nil {
			return errors.Wrapf(err, "%q", t.tbl.Table)
		}
		var (
			bp   = &b
			scan = bufio.NewScanner(rc)
			i    int
		)
		for scan.Scan() {
			i++
			err := t.values(bp, scan.Bytes())
			if err != nil {
				return fmt.Errorf("%q:%d: %s", file, i, err)
			}
		}
		if scan.Err() != nil {
			return fmt.Errorf("reading %q: %s", file, scan.Err())
		}
		if filepath.Base(file) == "hit_stats.jsonl" {
			for _, hc := range hitCounts {
				hitBulk.Values(site.ID, imp.pathIDs[hc.PathID], hc.Hour[:10]+" "+hc.Hour[11:19], hc.Count)
			}
			// Need to join errors from both bulk inserters, as one of them may
			// error which will cause the other to just issue "current
			// transaction is aborted"-errors.
			var (
				errs    = slices.Concat(errors.Unjoin(b.Finish()), errors.Unjoin(hitBulk.Finish()))
				newerrs = make([]error, 0, len(errs))
			)
			for _, e := range errs {
				if sErr, ok := errors.AsType[interface {
					Error() string
					SQLState() string
				}](e); ok && sErr.SQLState() != "25P02" {
					newerrs = append(newerrs, e)
				}
			}
			if len(newerrs) > 0 {
				return errors.Join(newerrs...)
			}
			// Everything was filtered (should never happen, but be safe).
			return errors.Join(errs...)
		} else {
			if err := b.Finish(); err != nil {
				return fmt.Errorf("inserting %q: %s", file, err)
			}
		}
	}
	return nil
}

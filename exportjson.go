package goatcounter

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zfilepath"
	"zgo.at/zstd/ztime"
)

const ExportJSONVersion = 2

// CreateJSON creates a new JSON export.
//
// Inserts a row in exports table and returns open file pointer to the
// destination file.
func (e *Export) CreateJSON(ctx context.Context) (*os.File, error) {
	site := MustGetSite(ctx)

	e.SiteID = site.ID
	e.CreatedAt = ztime.Now(ctx)
	e.Path = fmt.Sprintf("%s%sgoatcounter-export-%s-%s.zip",
		os.TempDir(), string(os.PathSeparator), site.Code,
		e.CreatedAt.Format("20060102T150405Z"))

	var err error
	e.ID, err = zdb.InsertID[ExportID](ctx, "export_id",
		`insert into exports (site_id, path, created_at, start_from_hit_id) values (?, ?, ?, ?)`,
		e.SiteID, e.Path, e.CreatedAt, e.StartFromHitID)
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

	tables := []struct {
		p string
		f func(w io.Writer) error
	}{
		{"paths", func(w io.Writer) error {
			return queryToJSON[Path](ctx, w, `select path_id, path, title, event from paths where site_id=?`, siteID)
		}},
		{"refs", func(w io.Writer) error {
			return queryToJSON[Ref](ctx, w, `
				select ref_id, ref, ref_scheme from refs
				where ref_id in (select ref_id from ref_counts where site_id=? group by ref_id)
				order by ref_id asc`, siteID)
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
				where site_id=? order by day asc`, siteID)
		}},
		{"system_stats", func(w io.Writer) error {
			return queryToJSON[ExportSystemStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, system_id, count
				from system_stats
				where site_id=? order by day asc`, siteID)
		}},
		{"location_stats", func(w io.Writer) error {
			return queryToJSON[ExportLocationStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, location, count
				from location_stats
				where site_id=? order by day asc`, siteID)
		}},
		{"size_stats", func(w io.Writer) error {
			return queryToJSON[ExportSizeStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, width, count
				from size_stats
				where site_id=? order by day asc`, siteID)
		}},
		{"language_stats", func(w io.Writer) error {
			return queryToJSON[ExportLanguageStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, language, count
				from language_stats
				where site_id=? order by day asc`, siteID)
		}},
		{"campaign_stats", func(w io.Writer) error {
			return queryToJSON[ExportCampaignStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, campaign_id, ref, count
				from campaign_stats
				where site_id=? order by day asc`, siteID)
		}},
		{"ref_stats", func(w io.Writer) error {
			return queryToJSON[ExportRefStat](ctx, w, `
				select hour, path_id, ref_id, total
				from ref_counts
				where site_id=? order by hour asc`, siteID)
		}},
		{"hit_stats", func(w io.Writer) error {
			return queryToJSON[ExportHitStat](ctx, w, `
				select substr(cast(day as text), 0, 11) as day, path_id, stats
				from hit_stats
				where site_id=? order by day asc`, siteID)
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
		j, _ := json.MarshalIndent(ExportInfo{
			Version:   ExportJSONVersion,
			CreatedAt: ztime.Now(ctx).Truncate(time.Second),
			Site:      GetSite(ctx).Display(ctx),
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
				return fmt.Errorf("table %q: %s", t.p, err)
			}
			err = t.f(w)
			if err != nil {
				return fmt.Errorf("table %q: %s", t.p, err)
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

func ImportJSON(ctx context.Context, fp *os.File, replace, email bool) (*time.Time, error) {
	site := MustGetSite(ctx)

	l := log.Module("import").With("site", site.ID, "replace", replace)
	l.Info(ctx, "import started")

	st, err := fp.Stat()
	if err != nil {
		return nil, err
	}

	z, err := zip.NewReader(fp, st.Size())
	if err != nil {
		return nil, err
	}

	dir := filepath.Base(fp.Name())
	dir, _ = zfilepath.SplitExt(dir)

	{ // Verify version
		f, err := z.Open(filepath.Join(dir, "info.json"))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		var info ExportInfo
		err = json.NewDecoder(f).Decode(&info)
		if err != nil {
			return nil, err
		}
		f.Close()

		if info.Version > ExportJSONVersion {
			return nil, fmt.Errorf("unknown export version %d; this version of GoatCounter (%s) only supports up to version %d",
				info.Version, Version, ExportJSONVersion)
		}
	}

	var (
		pathIDs    = make(map[PathID]PathID)
		refIDs     = make(map[RefID]RefID)
		browserIDs = make(map[BrowserID]BrowserID)
		systemIDs  = make(map[SystemID]SystemID)
	)
	dataTables := map[string]struct {
		table, idcol string
		cols         []string
		values       func(*zdb.BulkInsert, []byte) (int64, error)
	}{
		"paths.jsonl": {
			table: "paths",
			idcol: "path_id",
			cols:  []string{"site_id", "path", "title", "event"},
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
			table: "refs",
			idcol: "ref_id",
			cols:  []string{"ref", "ref_scheme"},
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
			table: "browsers",
			idcol: "browser_id",
			cols:  []string{"name", "version"},
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
			table: "systems",
			idcol: "system_id",
			cols:  []string{"name", "version"},
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
			table: "locations",
			idcol: "location_id",
			cols:  []string{"country", "region", "country_name", "region_name"},
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
	}

	err = zdb.TX(ctx, func(ctx context.Context) error {
		// First need to load the data tables
		for name, t := range dataTables {
			rc, err := z.Open(filepath.Join(dir, name))
			if err != nil {
				return fmt.Errorf("reading %q: %s", name, err)
			}

			var (
				b      = zdb.NewBulkInsert(ctx, t.table, t.cols)
				bp     = &b
				scan   = bufio.NewScanner(rc)
				i      int
				oldIDs = make([]int64, 0, 16)
			)
			if t.idcol != "" {
				b.Returning(t.idcol)
				// On ignore it won't return the ID column, so do a "fake update".
				//
				// TODO: on SQLite at least this will make the autoincrement skip
				// ID (has nothing to do with idcol being used here; happens on any
				// column). See if we can avoid that.
				//
				// TODO: doesn't work on refs due to NULL; need to un-null this
				// first. Ugh.
				//
				// sqlite> insert into refs2 (ref,ref_scheme) values ('XXX',null) returning ref_id;
				// │ ref_id │
				// │ 2      │
				//
				// sqlite> insert into refs2 (ref,ref_scheme) values ('XXX',null) returning ref_id;
				// │ ref_id │
				// │ 3      │
				b.OnConflict(`on conflict do update set ` + t.idcol + `=` + t.idcol)
			} else { // Just for languages
				b.OnConflict(`on conflict do nothing`)
			}
			for scan.Scan() {
				line := scan.Bytes()
				i++

				id, err := t.values(bp, line)
				if err != nil {
					return err
				}
				oldIDs = append(oldIDs, id)
			}
			if scan.Err() != nil {
				return fmt.Errorf("reading %q: %s", name, scan.Err())
			}
			err = b.Finish()
			if err != nil {
				return err
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
					pathIDs[PathID(oldIDs[i])] = PathID(n)
				case "refs":
					refIDs[RefID(oldIDs[i])] = RefID(n)
				case "browsers":
					browserIDs[BrowserID(oldIDs[i])] = BrowserID(n)
				case "systems":
					systemIDs[SystemID(oldIDs[i])] = SystemID(n)
				}
			}
		}

		// Insert stats
		statTables := map[string]struct {
			tbl    tbl
			values func(*zdb.BulkInsert, []byte) error
		}{
			"browser_stats": {
				tbl: Tables.BrowserStats,
				values: func(b *zdb.BulkInsert, line []byte) error {
					var v ExportBrowserStat
					err := json.Unmarshal(line, &v)
					if err != nil {
						return err
					}
					b.Values(site.ID, pathIDs[v.PathID], v.Day, browserIDs[v.BrowserID], v.Count)
					return nil
				},
			},
			"system_stats": {
				tbl: Tables.SystemStats,
				values: func(b *zdb.BulkInsert, line []byte) error {
					var v ExportSystemStat
					err := json.Unmarshal(line, &v)
					if err != nil {
						return err
					}
					b.Values(site.ID, pathIDs[v.PathID], v.Day, systemIDs[v.SystemID], v.Count)
					return nil
				},
			},
			"location_stats": {
				tbl: Tables.LocationStats,
				values: func(b *zdb.BulkInsert, line []byte) error {
					var v ExportLocationStat
					err := json.Unmarshal(line, &v)
					if err != nil {
						return err
					}
					b.Values(site.ID, pathIDs[v.PathID], v.Day, v.Location, v.Count)
					return nil
				},
			},
			"size_stats": {
				tbl: Tables.SizeStats,
				values: func(b *zdb.BulkInsert, line []byte) error {
					var v ExportSizeStat
					err := json.Unmarshal(line, &v)
					if err != nil {
						return err
					}
					b.Values(site.ID, pathIDs[v.PathID], v.Day, v.Width, v.Count)
					return nil
				},
			},
			"language_stats": {
				tbl: Tables.LanguageStats,
				values: func(b *zdb.BulkInsert, line []byte) error {
					var v ExportLanguageStat
					err := json.Unmarshal(line, &v)
					if err != nil {
						return err
					}
					b.Values(site.ID, pathIDs[v.PathID], v.Day, v.Language, v.Count)
					return nil
				},
			},
			// TODO: need to export campaigns table also
			//"campaign_stats": {
			//	tbl: Tables.CampaignStats,
			//	values: func(b *zdb.BulkInsert, line []byte) error {
			//		var v ExportCampaignStat
			//		err := json.Unmarshal(line, &v)
			//		if err != nil {
			//			return err
			//		}
			//		b.Values(site.ID, pathIDs[v.PathID], v.Day, v.CampaignID, v.Ref, v.Count)
			//		return nil
			//	},
			//},
			"ref_stats": {
				tbl: Tables.RefCounts,
				values: func(b *zdb.BulkInsert, line []byte) error {
					var v ExportRefStat
					err := json.Unmarshal(line, &v)
					if err != nil {
						return err
					}
					b.Values(
						site.ID, pathIDs[v.PathID],
						v.Hour[:10]+" "+v.Hour[11:19],
						refIDs[v.RefID], v.Count)
					return nil
				},
			},
			// TODO: this also need to do hit_counts
			// TODO: doesn't work on SQLite
			// "hit_stats": {
			// 	tbl: Tables.HitStats,
			// 	values: func(b *zdb.BulkInsert, line []byte) error {
			// 		var v ExportHitStat
			// 		err := json.Unmarshal(line, &v)
			// 		if err != nil {
			// 			return err
			// 		}
			// 		b.Dump(zdb.DumpQuery)
			// 		b.Values(site.ID, pathIDs[v.PathID], v.Day, v.Stats)
			// 		return nil
			// 	},
			// },
		}
		for name, t := range statTables {
			rc, err := z.Open(filepath.Join(dir, name) + ".jsonl")
			if err != nil {
				return fmt.Errorf("reading %q: %s", name, err)
			}

			var (
				b    = t.tbl.Bulk(ctx)
				bp   = &b
				scan = bufio.NewScanner(rc)
				i    int
			)
			for scan.Scan() {
				line := scan.Bytes()
				i++

				err := t.values(bp, line)
				if err != nil {
					return fmt.Errorf("reading %q:%d: %s", name, i, err)
				}
			}
			if scan.Err() != nil {
				return fmt.Errorf("reading %q: %s", name, scan.Err())
			}
			err = b.Finish()
			if err != nil {
				return fmt.Errorf("reading %q: %s", name, err)
			}
		}

		return nil
	})

	return nil, err
}

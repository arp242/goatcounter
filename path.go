package goatcounter

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2/pkg/db2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zreflect"
)

type PathID int32

type Path struct {
	ID    PathID     `db:"path_id,id" json:"id"` // Path ID
	Site  SiteID     `db:"site_id" json:"-"`
	Path  string     `db:"path" json:"path"`   // Path name
	Title string     `db:"title" json:"title"` // Page title
	Event zbool.Bool `db:"event" json:"event"` // Is this an event?
}

func (Path) Table() string { return "paths" }

var _ zdb.Defaulter = &Path{}

func (p *Path) Defaults(ctx context.Context) {
	if p.Site == 0 {
		p.Site = MustGetSite(ctx).ID
	}
}

var _ zdb.Validator = &Path{}

func (p *Path) Validate(ctx context.Context) error {
	v := NewValidate(ctx)
	v.Required("site_id", p.Site)
	v.UTF8("path", p.Path)
	v.UTF8("title", p.Title)
	v.Len("path", p.Path, 1, 2048)
	v.Len("title", p.Title, 0, 1024)
	return v.ErrorOrNil()
}

func (p *Path) ByID(ctx context.Context, id PathID) error {
	err := zdb.Get(ctx, p,
		`/* Path.ByID */ select * from paths where path_id=? and site_id=?`,
		id, MustGetSite(ctx).ID)
	return errors.Wrapf(err, "Path.ByID(%d)", id)
}

func (p *Path) ByPath(ctx context.Context, path string) error {
	err := zdb.Get(ctx, p,
		`/* Path.ByPath */ select * from paths where site_id=? and lower(path) = lower(?)`,
		MustGetSite(ctx).ID, path)
	return errors.Wrapf(err, "Path.ByPath(%q)", path)
}

func (p *Path) GetOrInsert(ctx context.Context) error {
	site := MustGetSite(ctx)
	title := p.Title
	k := strconv.Itoa(int(site.ID)) + p.Path
	c, ok := cachePaths(ctx).Get(k)
	if ok {
		*p = c
		cachePaths(ctx).Touch(k)

		err := p.updateTitle(ctx, p.Title, title)
		if err != nil {
			log.Error(ctx, err, "path_id", p.ID, "title", title)
		}
		return nil
	}

	p.Defaults(ctx)
	err := p.Validate(ctx)
	if err != nil {
		return errors.Wrap(err, "Path.GetOrInsert")
	}

	err = zdb.Get(ctx, p, `/* Path.GetOrInsert */
		select * from paths
		where site_id = $1 and lower(path) = lower($2)
		limit 1`, site.ID, p.Path)
	if err != nil && !zdb.ErrNoRows(err) {
		return errors.Errorf("Path.GetOrInsert select: %w", err)
	}
	if err == nil {
		err := p.updateTitle(ctx, p.Title, title)
		if err != nil {
			log.Error(ctx, err, "path_id", p.ID, "title", title)
		}
		cachePaths(ctx).Set(k, *p)
		return nil
	}

	// Insert new path.
	err = zdb.Insert(ctx, p)
	if err != nil {
		return errors.Wrap(err, "Path.GetOrInsert insert")
	}

	// Make sure to update any filters.
	var f Filters
	err = f.List(ctx)
	if err != nil {
		return errors.Wrap(err, "Path.GetOrInsert insert")
	}
	for _, ff := range f {
		m := ff.Match(p.Path, p.Title, bool(p.Event))
		if ff.Invert {
			m = !m
		}
		if m {
			err := ff.Append(ctx, p.ID)
			if err != nil {
				return errors.Wrap(err, "Path.GetOrInsert append filter")
			}
		}
	}

	cachePaths(ctx).Set(k, *p)
	return nil
}

func (p Path) updateTitle(ctx context.Context, currentTitle, newTitle string) error {
	if newTitle == currentTitle {
		return nil
	}

	k := strconv.Itoa(int(p.ID))
	_, ok := cacheChangedTitles(ctx).Get(k)
	if !ok {
		cacheChangedTitles(ctx).Set(k, []string{newTitle})
		return nil
	}

	var titles []string
	cacheChangedTitles(ctx).Modify(k, func(v []string) []string {
		v = append(v, newTitle)
		titles = v
		return v
	})

	grouped := make(map[string]int)
	for _, t := range titles {
		grouped[t]++
	}

	for t, n := range grouped {
		if n > 10 {
			err := zdb.Exec(ctx, `update paths set title = $1 where path_id = $2`, t, p.ID)
			if err != nil {
				return errors.Wrap(err, "Paths.updateTitle")
			}
			cacheChangedTitles(ctx).Delete(k)
			break
		}
	}

	return nil
}

// Merge the given paths in to this one.
func (p Path) Merge(ctx context.Context, paths Paths) error {
	pathIDs := make([]PathID, 0, len(paths))
	for _, pp := range paths {
		if pp.ID == p.ID { // Shouldn't happen, but just in case.
			return fmt.Errorf("Path.Merge: destination ID %d also in paths to merge", p.ID)
		}
		pathIDs = append(pathIDs, pp.ID)
	}

	siteID := MustGetSite(ctx).ID
	err := zdb.TX(ctx, func(ctx context.Context) error {
		// Update stats and counts tables, except hit_stats
		for _, tt := range zreflect.Values(Tables, "", "") {
			var (
				t      = tt.(tbl)
				i      = slices.Index(t.Columns, "path_id")
				sel    = append([]string{}, t.Columns...)
				selCTE = append([]string{}, t.Columns...)
				group  = append([]string{}, t.Columns...)
			)
			if t.Table == "hit_stats" {
				continue
			}

			sel[i] = ":path_id"
			selCTE = slices.Delete(selCTE, i, i+1)
			l := len(selCTE) - 1

			selCTE[l] = fmt.Sprintf("sum(%[1]s) as %[1]s", selCTE[l])

			group = append(group[i+1:len(group)-1], "site_id")

			err := zdb.Exec(ctx, `load:paths.Merge`, map[string]any{
				"Table":      t.Table,
				"SelectCTE":  strings.Join(selCTE, ", "),
				"Select":     strings.Join(sel, ", "),
				"Columns":    strings.Join(t.Columns, ", "),
				"OnConflict": t.OnConflict(ctx),
				"Group":      strings.Join(group, ", "),
				"path_id":    p.ID,
				"site_id":    siteID,
				"paths":      db2.Array(ctx, pathIDs),
				"in":         db2.In(ctx),
			})
			if err != nil {
				return err
			}
			err = zdb.Exec(ctx, `/* Path.Merge */
				delete from :tbl where site_id=:site_id and path_id :in (:paths)`,
				map[string]any{
					"tbl":     zdb.SQL(t.Table),
					"site_id": siteID,
					"paths":   db2.Array(ctx, pathIDs),
					"in":      db2.In(ctx),
				})
			if err != nil {
				return err
			}
		}

		// Update hit_stats; for PostgreSQL we can update inline, for SQLite we
		// need to also select and delete the merge target and re-insert it.
		loadPathIDs := append([]PathID{}, pathIDs...)
		if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
			loadPathIDs = append(loadPathIDs, p.ID)
		}
		var hitStats []struct {
			Day   string `db:"day"`
			Stats []byte `db:"stats"`
		}
		err := zdb.Select(ctx, &hitStats, `load:paths.Merge-hit_stats`, map[string]any{
			"site_id": siteID,
			"paths":   db2.Array(ctx, loadPathIDs),
			"in":      db2.In(ctx),
		})
		if err != nil {
			return err
		}
		err = zdb.Exec(ctx, `/* Path.Merge */
			delete from hit_stats where site_id=:site_id and path_id :in (:paths)`,
			map[string]any{
				"site_id": siteID,
				"paths":   db2.Array(ctx, pathIDs),
				"in":      db2.In(ctx),
			})
		if err != nil {
			return err
		}

		ins := Tables.HitStats.Bulk(ctx)
		if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
			// Reset the "on conflict", which SQLite doesn't support for
			// hit_stats. We deleted and fetched the target (for SQLite) before,
			// so that's okay.
			ins = zdb.NewBulkInsert(ctx, "hit_stats", []string{"site_id", "path_id", "day", "stats"})
		}
		for _, d := range hitStats {
			var ru [][]int
			zjson.MustUnmarshal(d.Stats, &ru)
			for _, s := range ru[1:] {
				for i := range s {
					ru[0][i] += s[i]
				}
			}
			ins.Values(siteID, p.ID, d.Day[:10], zjson.MustMarshal(ru[0]))
		}
		if err := ins.Finish(); err != nil {
			return err
		}

		// Update hits and delete old paths.
		err = zdb.Exec(ctx, `/* Path.Merge */
			update hits set path_id=:path_id where site_id=:site_id and path_id :in (:paths)`,
			map[string]any{
				"site_id": siteID,
				"path_id": p.ID,
				"paths":   db2.Array(ctx, pathIDs),
				"in":      db2.In(ctx),
			})
		if err != nil {
			return err
		}
		return zdb.Exec(ctx, `/* Path.Merge */
			delete from paths where site_id=:site_id and path_id :in (:paths)`,
			map[string]any{
				"site_id": siteID,
				"paths":   db2.Array(ctx, pathIDs),
				"in":      db2.In(ctx),
			})
	})
	return errors.Wrapf(err, "Path.Merge(%d, %v)", p.ID, pathIDs)
}

type Paths []Path

// List all paths for a site.
func (p *Paths) List(ctx context.Context, siteID SiteID, after PathID, limit int) (bool, error) {
	err := zdb.Select(ctx, p, "load:paths.List", map[string]any{
		"site":  siteID,
		"after": after,
		"limit": limit + 1,
	})
	if err != nil {
		return false, errors.Wrap(err, "Paths.List")
	}

	more := len(*p) > limit
	if more {
		pp := *p
		pp = pp[:len(pp)-1]
		*p = pp
	}
	return more, nil
}

// FindPathsIDs finds path IDs by exact matches on the name.
func FindPathIDs(ctx context.Context, list []string) ([]PathID, error) {
	var paths []PathID
	err := zdb.Select(ctx, &paths, `/* FindPathIDs */
		select path_id from paths where site_id=:site_id and lower(path) :in (:paths)`,
		map[string]any{
			"site_id": MustGetSite(ctx).ID,
			"paths":   db2.ArrayString(ctx, list),
			"in":      db2.In(ctx),
		})
	return paths, errors.Wrap(err, "FindPathIDs")
}

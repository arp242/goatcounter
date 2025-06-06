package goatcounter

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2/log"
	"zgo.at/zcache"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zreflect"
)

type Path struct {
	ID    int64      `db:"path_id" json:"id"` // Path ID
	Site  int64      `db:"site_id" json:"-"`
	Path  string     `db:"path" json:"path"`   // Path name
	Title string     `db:"title" json:"title"` // Page title
	Event zbool.Bool `db:"event" json:"event"` // Is this an event?
}

func (p *Path) Defaults(ctx context.Context) {}

func (p *Path) Validate(ctx context.Context) error {
	v := NewValidate(ctx)

	v.UTF8("path", p.Path)
	v.UTF8("title", p.Title)
	v.Len("path", p.Path, 1, 2048)
	v.Len("title", p.Title, 0, 1024)

	return v.ErrorOrNil()
}

func (p *Path) ByID(ctx context.Context, id int64) error {
	return errors.Wrapf(zdb.Get(ctx, p,
		`/* Path.ByID */ select * from paths where path_id=? and site_id=?`,
		id, MustGetSite(ctx).ID), "Path.ByID %d", id)
}

func (p *Path) GetOrInsert(ctx context.Context) error {
	site := MustGetSite(ctx)
	title := p.Title
	k := strconv.FormatInt(site.ID, 10) + p.Path
	c, ok := cachePaths(ctx).Get(k)
	if ok {
		*p = c.(Path)
		cachePaths(ctx).Touch(k, zcache.DefaultExpiration)

		err := p.updateTitle(ctx, p.Title, title)
		if err != nil {
			log.Error(ctx, err, "path_id", p.ID, "title", title)
		}
		return nil
	}

	p.Defaults(ctx)
	err := p.Validate(ctx)
	if err != nil {
		return err
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
		cachePaths(ctx).SetDefault(k, *p)
		return nil
	}

	// Insert new row.
	p.ID, err = zdb.InsertID(ctx, "path_id",
		`insert into paths (site_id, path, title, event) values (?, ?, ?, ?)`,
		site.ID, p.Path, p.Title, p.Event)
	if err != nil {
		return errors.Wrap(err, "Path.GetOrInsert insert")
	}

	cachePaths(ctx).SetDefault(k, *p)
	return nil
}

func (p Path) updateTitle(ctx context.Context, currentTitle, newTitle string) error {
	if newTitle == currentTitle {
		return nil
	}

	k := strconv.FormatInt(p.ID, 10)
	_, ok := cacheChangedTitles(ctx).Get(k)
	if !ok {
		cacheChangedTitles(ctx).SetDefault(k, []string{newTitle})
		return nil
	}

	var titles []string
	cacheChangedTitles(ctx).Modify(k, func(v any) any {
		vv := v.([]string)
		vv = append(vv, newTitle)
		titles = vv
		return vv
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
	pathIDs := make([]int64, 0, len(paths))
	for _, pp := range paths {
		if pp.ID == p.ID { // Shouldn't happen, but just in case.
			return fmt.Errorf("Path.Merge: destination ID %d also in paths to merge", p.ID)
		}
		pathIDs = append(pathIDs, pp.ID)
	}

	siteID := MustGetSite(ctx).ID
	err := zdb.TX(ctx, func(ctx context.Context) error {
		for _, tt := range zreflect.Values(Tables, "", "") {
			t := tt.(tbl)

			sel := append([]string{}, t.Columns...)
			sel[slices.Index(sel, "path_id")] = ":path_id"
			q := fmt.Sprintf(`
				insert into %[1]s (%[2]s)
					select %[3]s from %[1]s
					where site_id = :site_id and path_id in (:paths)
				%[4]s`,
				t.Table, strings.Join(t.Columns, ", "),
				strings.Join(sel, ", "), t.OnConflict(ctx))

			err := zdb.Exec(ctx, q, map[string]any{
				"path_id": p.ID,
				"site_id": siteID,
				"paths":   pathIDs,
			})
			if err != nil {
				return err
			}
		}

		err := zdb.Exec(ctx, `update hits set path_id = ? where site_id = ? and path_id in (?)`,
			p.ID, siteID, pathIDs)
		if err != nil {
			return err
		}
		return zdb.Exec(ctx, `delete from paths where site_id = ? and path_id in (?)`, siteID, pathIDs)
	})
	return errors.Wrap(err, "Path.Merge")
}

type Paths []Path

// List all paths for a site.
func (p *Paths) List(ctx context.Context, siteID, after int64, limit int) (bool, error) {
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

// PathFilter returns a list of IDs matching the path name.
//
// if matchTitle is true it will match the title as well.
func PathFilter(ctx context.Context, filter string, matchTitle bool) ([]int64, error) {
	var paths []int64
	err := zdb.Select(ctx, &paths, "load:paths.PathFilter", map[string]any{
		"site":        MustGetSite(ctx).ID,
		"filter":      "%" + filter + "%",
		"match_title": matchTitle,
	})
	if err != nil {
		return nil, errors.Wrap(err, "PathFilter")
	}

	// Nothing matches: make sure there's a slice with an invalid path_id, so
	// the queries using the result don't select anything.
	if len(paths) == 0 {
		paths = []int64{-1}
	}
	return paths, nil
}

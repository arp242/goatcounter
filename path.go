// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"strconv"
	"time"

	"zgo.at/errors"
	"zgo.at/zcache"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

type Path struct {
	ID   int64 `db:"path_id"`
	Site int64 `db:"site_id"`

	Path  string   `db:"path"`
	Title string   `db:"title"`
	Event zdb.Bool `db:"event"`
}

func (p *Path) Defaults(ctx context.Context) {
}

func (p *Path) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.UTF8("path", p.Path)
	v.UTF8("title", p.Title)
	v.Len("path", p.Path, 1, 2048)
	v.Len("title", p.Title, 0, 1024)

	return v.ErrorOrNil()
}

var cachePaths = zcache.New(1*time.Hour, 5*time.Minute)

func (p *Path) GetOrInsert(ctx context.Context) error {
	site := MustGetSite(ctx)
	title := p.Title
	k := strconv.FormatInt(site.ID, 10) + p.Path
	c, ok := cachePaths.Get(k)
	if ok {
		*p = c.(Path)
		cachePaths.Touch(k, zcache.DefaultExpiration)

		err := p.updateTitle(ctx, p.Title, title)
		if err != nil {
			zlog.Fields(zlog.F{
				"path_id": p.ID,
				"title":   title,
			}).Error(err)
		}
		return nil
	}

	db := zdb.MustGet(ctx)

	p.Defaults(ctx)
	err := p.Validate(ctx)
	if err != nil {
		return err
	}

	row := db.QueryRowxContext(ctx, `/* Path.GetOrInsert */
		select * from paths
		where site_id=$1 and lower(path)=lower($2)
		limit 1`,
		site.ID, p.Path)
	if row.Err() != nil {
		return errors.Errorf("Path.GetOrInsert select: %w", row.Err())
	}
	err = row.StructScan(p)
	if err != nil && !zdb.ErrNoRows(err) {
		return errors.Errorf("Path.GetOrInsert select: %w", err)
	}
	if err == nil {
		err := p.updateTitle(ctx, p.Title, title)
		if err != nil {
			zlog.Fields(zlog.F{
				"path_id": p.ID,
				"title":   title,
			}).Error(err)
		}
		cachePaths.SetDefault(k, *p)
		return nil
	}

	// Insert new row.
	p.ID, err = zdb.InsertID(ctx, "path_id",
		`insert into paths (site_id, path, title, event) values ($1, $2, $3, $4)`,
		site.ID, p.Path, p.Title, p.Event)
	if err != nil {
		return errors.Wrap(err, "Path.GetOrInsert insert")
	}

	cachePaths.SetDefault(k, *p)
	return nil
}

var changedTitles = zcache.New(48*time.Hour, 1*time.Hour)

func (p Path) updateTitle(ctx context.Context, currentTitle, newTitle string) error {
	if newTitle == currentTitle {
		return nil
	}

	k := strconv.FormatInt(p.ID, 10)
	_, ok := changedTitles.Get(k)
	if !ok {
		changedTitles.SetDefault(k, []string{newTitle})
		return nil
	}

	var titles []string
	changedTitles.Modify(k, func(v interface{}) interface{} {
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
			_, err := zdb.MustGet(ctx).ExecContext(ctx,
				`update paths set title=$1 where path_id=$2`,
				t, p.ID)
			if err != nil {
				return errors.Wrap(err, "Paths.updateTitle")
			}
			changedTitles.Delete(k)
			break
		}
	}

	return nil
}

// PathFilter returns a list of IDs matching the path name.
//
// if matchTitle is true it will match the title as well.
func PathFilter(ctx context.Context, filter string, matchTitle bool) ([]int64, error) {
	var paths []int64
	err := zdb.QuerySelect(ctx, &paths, `/* PathFilter */
		select path_id from paths
		where
			site_id = :site and (
				lower(path) like lower(:filter)
				{{:match_title or lower(title) like lower(:filter)}}
			)
		limit 65500`,
		zdb.A{
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

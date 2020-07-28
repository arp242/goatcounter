// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"

	"zgo.at/errors"
	"zgo.at/zdb"
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
	// 	p.cleanPath(ctx)
}

func (p *Path) Validate(ctx context.Context) error {
	v := zvalidate.New()

	v.UTF8("path", p.Path)
	v.UTF8("title", p.Title)
	v.Len("path", p.Path, 1, 2048)
	v.Len("title", p.Title, 0, 1024)

	return v.ErrorOrNil()
}

// TODO: update title once a day or something?
func (p *Path) GetOrInsert(ctx context.Context) error {
	db := zdb.MustGet(ctx)
	site := MustGetSite(ctx)

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
		return nil
	}

	// Insert new row.
	p.ID, err = insertWithID(ctx, "path_id",
		`insert into paths (site_id, path, title, event) values ($1, $2, $3, $4)`,
		site.ID, p.Path, p.Title, p.Event)
	return errors.Wrap(err, "Path.GetOrInsert insert")
}

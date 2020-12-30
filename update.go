// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

/*
Adding a new update:

insert into updates(created_at, show_at, subject, body) values (now(), now(),
    'subject', '<p>
	body</p>
	');
*/

package goatcounter

import (
	"context"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

type Update struct {
	ID        int64     `db:"id"`
	Subject   string    `db:"subject"`
	Body      string    `db:"body"`
	CreatedAt time.Time `db:"created_at"`
	ShowAt    time.Time `db:"show_at"`
	Seen      bool      `db:"-"`
}

type Updates []Update

// HasSince reports if there are any updates since the given date.
func (u *Updates) HasSince(ctx context.Context, since time.Time) (bool, error) {
	var has bool
	err := zdb.Get(ctx, &has, `select 1 from updates where show_at >= $1`, since)
	if zdb.ErrNoRows(err) {
		err = nil
	}
	return has, errors.Wrap(err, "Updates.HasSince")
}

// List all updates.
func (u *Updates) List(ctx context.Context, since time.Time) error {
	err := zdb.Select(ctx, u, `select * from updates order by show_at desc`)
	if err != nil {
		return errors.Wrap(err, "Updates.List")
	}

	uu := *u
	for i := range uu {
		if since.After(uu[i].ShowAt) {
			break
		}
		uu[i].Seen = true
	}
	return nil
}

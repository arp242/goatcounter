// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"time"

	"zgo.at/goatcounter/errors"
	"zgo.at/zdb"
)

/* Adding a new update:

insert into updates(created_at, show_at, subject, body) values (now(), now(),
    "...", "...");

update users set unseen_updates=1;
*/
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
	err := zdb.MustGet(ctx).GetContext(ctx, &has,
		`select 1 from updates where show_at >= $1`, since)
	if zdb.ErrNoRows(err) {
		err = nil
	}
	return has, errors.Wrap(err, "Updates.ListUnseen")
}

// Lists all updates.
func (u *Updates) List(ctx context.Context, since time.Time) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, u, `select * from updates order by show_at desc`)
	if err != nil {
		return errors.Wrap(err, "Updates.ListUnseen")
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

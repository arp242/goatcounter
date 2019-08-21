// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/validate"
	"zgo.at/zlog"
)

type Browser struct {
	Site   int64 `db:"site"`
	Domain int64 `db:"domain"`

	DomainName string `db:"-"`
	Browser    string `db:"browser"`

	CreatedAt time.Time `db:"created_at"`
}

// Defaults sets fields to default values, unless they're already set.
func (b *Browser) Defaults(ctx context.Context) {
	site := MustGetSite(ctx)
	b.Site = site.ID

	// Load domain.
	if b.Domain == 0 {
		var d Domain
		err := d.ByNameOrFirst(ctx, b.DomainName)
		if err != nil {
			zlog.Error(err)
		}
		b.Domain = d.ID
	}

	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
}

// Validate the object.
func (b *Browser) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", b.Site)

	return v.ErrorOrNil()
}

// Insert a new row.
func (b *Browser) Insert(ctx context.Context) error {
	b.Defaults(ctx)
	err := b.Validate(ctx)
	if err != nil {
		return err
	}

	_, err = MustGetDB(ctx).ExecContext(ctx, `insert into browsers (site, browser, created_at)
		values ($1, $2, $3)`, b.Site, b.Browser, sqlDate(b.CreatedAt))
	return errors.Wrap(err, "Browser.Insert")
}

type Browsers []Browser

func (b *Browsers) List(ctx context.Context) error {
	return errors.Wrap(MustGetDB(ctx).SelectContext(ctx, b,
		`select * from browsers where site=$1`, MustGetSite(ctx).ID),
		"Browsers.List")
}

type BrowserStats []struct {
	Browser string
	Count   int
}

func (h *BrowserStats) List(ctx context.Context, start, end time.Time) error {
	site := MustGetSite(ctx)
	err := MustGetDB(ctx).SelectContext(ctx, h, `
		select browser, count(browser) as count
		from browsers
		where
			site=$1 and
			created_at >= $2 and
			created_at <= $3
		group by browser
		order by count desc
		limit $4`,
		site.ID, dayStart(start), dayEnd(end), site.Settings.Limits.Ref)
	return errors.Wrap(err, "BrowserStats.List")
}

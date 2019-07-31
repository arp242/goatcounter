package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/validate"
)

type Browser struct {
	Site      int64     `db:"site"`
	Browser   string    `db:"browser"`
	CreatedAt time.Time `db:"created_at"`
}

// Defaults sets fields to default values, unless they're already set.
func (b *Browser) Defaults(ctx context.Context) {
	// TODO: not doing this as it's not set from memstore.
	// site := MustGetSite(ctx)
	// b.Site = site.ID

	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
}

// Validate the object.
func (b *Browser) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", b.Site)
	v.Required("browser", b.Browser)

	return v.ErrorOrNil()
}

// Insert a new row.
func (b *Browser) Insert(ctx context.Context) error {
	b.Defaults(ctx)
	err := b.Validate(ctx)
	if err != nil {
		return err
	}

	db := MustGetDB(ctx)
	_, err = db.ExecContext(ctx, `insert into browsers (site, browser, created_at)
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
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, h, `
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

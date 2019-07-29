package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/validate"
)

type BrowserStat struct {
	Site      int64     `db:"site"`
	Browser   string    `db:"browser"`
	CreatedAt time.Time `db:"created_at"`
}

// Defaults sets fields to default values, unless they're already set.
func (b *BrowserStat) Defaults(ctx context.Context) {
	// TODO: not doing this as it's not set from memstore.
	// site := MustGetSite(ctx)
	// b.Site = site.ID

	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
}

// Validate the object.
func (b *BrowserStat) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", b.Site)
	v.Required("browser", b.Browser)

	return v.ErrorOrNil()
}

// Insert a new row.
func (b *BrowserStat) Insert(ctx context.Context) error {
	b.Defaults(ctx)
	err := b.Validate(ctx)
	if err != nil {
		return err
	}

	db := MustGetDB(ctx)
	_, err = db.ExecContext(ctx, `insert into browser_stats (site, browser, created_at)
		values ($1, $2, $3)`, b.Site, b.Browser, b.CreatedAt)
	return errors.Wrap(err, "BrowserStat.Insert")
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
		from browser_stats
		where
			site=$1 and
			date(created_at) >= $2 and
			date(created_at) <= $3
		group by browser
		order by count desc
		limit 50`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	return errors.Wrap(err, "BrowserStats.List")
}

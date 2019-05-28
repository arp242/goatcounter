package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/teamwork/validate"
)

type Hit struct {
	Site int64 `db:"site" json:"-"`

	Path      string    `db:"path" json:"p"`
	Ref       string    `db:"ref" json:"r"`
	CreatedAt time.Time `db:"created_at"`
}

// Defaults sets fields to default values, unless they're already set.
func (h *Hit) Defaults(ctx context.Context) {
	site := MustGetSite(ctx)
	h.Site = site.ID

	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now().UTC()
	}
}

// Validate the object.
func (h *Hit) Validate(ctx context.Context) error {
	v := validate.New()

	v.Required("site", h.Site)
	v.Required("path", h.Path)

	return v.ErrorOrNil()
}

// Insert a new row.
func (h *Hit) Insert(ctx context.Context) error {
	db := MustGetDB(ctx)
	h.Defaults(ctx)
	err := h.Validate(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `insert into hits (site, path, ref) values ($1, $2, $3)`,
		h.Site, h.Path, h.Ref)
	return errors.Wrap(err, "Site.Insert")
}

type Hits []Hit

func (h *Hits) List(ctx context.Context) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)
	err := db.SelectContext(ctx, h, `select * from hits
		where site=$1 order by created_at desc limit 500;`, site.ID)
	return errors.Wrap(err, "Hits.List")
}

type HitStat struct {
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	Count     int       `db:"count" json:"count"`
}

type HitStats []HitStat

func (s *HitStat) Daily(ctx context.Context, days int) (int, error) {
	return 0, nil
}

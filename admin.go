package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

type AdminStat struct {
	Code      string    `db:"code"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	Count     int       `db:"count"`
}

type AdminStats []AdminStat

// List stats for all sites, for all time.
func (a *AdminStats) List(ctx context.Context) error {
	err := MustGetDB(ctx).SelectContext(ctx, a, `
		select
			sites.code,
			sites.name,
			sites.created_at,
			count(*) as count
		from hits
		join sites on hits.site=sites.id
		group by sites.code, sites.name, sites.created_at
		order by count desc`)
	return errors.Wrap(err, "AdminStats.List")
}

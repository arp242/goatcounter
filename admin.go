// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

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
			count(*) - 1 as count
		from sites
		left join hits on hits.site=sites.id
		group by sites.code, sites.name, sites.created_at
		order by count desc`)
	return errors.Wrap(err, "AdminStats.List")
}

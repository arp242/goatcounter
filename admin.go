// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
)

type AdminStat struct {
	Code      string    `db:"code"`
	Name      string    `db:"name"`
	User      string    `db:"user"`
	Email     string    `db:"email"`
	Public    bool      `db:"public"`
	CreatedAt time.Time `db:"created_at"`
	Plan      string    `db:"plan"`
	Count     int       `db:"count"`
}

type AdminStats []AdminStat

// List stats for all sites, for all time.
func (a *AdminStats) List(ctx context.Context) error {
	// Needs --tags json1: too much work.
	//js := "json_extract(settings, '$.public')"
	js := "'0'"
	if cfg.PgSQL {
		js = "settings::json->>'public'"
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			sites.code,
			sites.name,
			sites.created_at,
			sites.plan,
			users.name as user,
			users.email,
			%s as public,
			count(*) - 1 as count
		from sites
		left join hits on hits.site=sites.id
		join users on users.site=sites.id
		group by sites.code, sites.name, sites.created_at, users.name, users.email, public, plan
		order by count desc`, js))
	return errors.Wrap(err, "AdminStats.List")
}

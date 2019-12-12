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
	ID        int64     `db:"id"`
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
	// TODO: needs --tags json1: too much work for now.
	//js := "json_extract(settings, '$.public')"
	js := "'0'"
	if cfg.PgSQL {
		js = "settings::json->>'public'"
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			sites.id,
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
		where hits.created_at >= now() - interval '30 days'
		group by sites.id, sites.code, sites.name, sites.created_at, users.name, users.email, public, plan
		order by count desc`, js))
	return errors.Wrap(err, "AdminStats.List")
}

type AdminSiteStat struct {
	Site           Site `db:"-"`
	User           User `db:"-"`
	CountTotal     int  `db:"count_total"`
	CountLastMonth int  `db:"count_last_month"`
	CountPrevMonth int  `db:"count_prev_month"`
}

// ByID gets stats for a single site.
func (a *AdminSiteStat) ByID(ctx context.Context, id int64) error {
	err := a.Site.ByID(ctx, id)
	if err != nil {
		return err
	}

	err = a.User.BySite(ctx, id)
	if err != nil {
		return err
	}

	err = zdb.MustGet(ctx).GetContext(ctx, a, `
		select
			(select count(*) from hits where site=$1) as count_total,
			(select count(*) from hits where site=$1
				and created_at >= now() - interval '30 days') as count_last_month,
			(select count(*) from hits where site=$1
				and created_at >= now() - interval '60 days'
				and created_at <= now() - interval '30 days'
			) as count_prev_month
		`, id)

	return errors.Wrap(err, "AdminSiteStats.ByID")
}

type AdminCountRef struct {
	Site     int64  `db:"site"`
	CountRef string `db:"count_ref"`
	Count    int    `db:"count"`
}

type AdminCountRefs []AdminCountRef

func (a *AdminCountRefs) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, a, `
		select site, count_ref, count(*) as count
		from hits where count_ref != ''
		group by site, count_ref
		order by count desc`), "AdminCountRefs")
}

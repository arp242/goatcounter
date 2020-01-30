// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
)

type AdminStat struct {
	ID         int64     `db:"id"`
	Parent     *int64    `db:"parent"`
	Code       string    `db:"code"`
	Name       string    `db:"name"`
	LinkDomain string    `db:"link_domain"`
	User       string    `db:"user"`
	Email      string    `db:"email"`
	Public     bool      `db:"public"`
	CreatedAt  time.Time `db:"created_at"`
	Plan       string    `db:"plan"`
	Count      int       `db:"count"`
}

type AdminStats []AdminStat

// List stats for all sites, for all time.
func (a *AdminStats) List(ctx context.Context, order string) error {
	if order == "" || !sliceutil.InStringSlice([]string{"count", "created_at"}, order) {
		order = "count"
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			sites.id,
			sites.parent,
			sites.code,
			sites.name,
			sites.created_at,
			sites.plan,
			sites.link_domain,
			users.name as user,
			users.email,
			count(*) - 1 as count
		from sites
		left join hits on hits.site=sites.id
		left join users on users.site=coalesce(sites.parent, sites.id)
		where hits.created_at >= now() - interval '30 days'
		group by sites.id, sites.code, sites.name, sites.created_at, users.name, users.email, plan
		having count(*) > 1000
		order by %s desc`, order))
	if err != nil {
		return errors.Wrap(err, "AdminStats.List")
	}

	// Add all the child plan counts to the parents.
	aa := *a
	for _, s := range aa {
		if s.Plan != PlanChild {
			continue
		}

		for i, s2 := range aa {
			if s2.ID == *s.Parent {
				aa[i].Count += s.Count
				break
			}
		}
	}
	if order == "count" {
		sort.Slice(aa, func(i, j int) bool { return aa[i].Count > aa[j].Count })
	}

	return nil
}

type AdminSiteStat struct {
	Site           Site        `db:"-"`
	User           User        `db:"-"`
	Usage          AdminUsages `db:"-"`
	LastData       time.Time   `db:"last_data"`
	CountTotal     int         `db:"count_total"`
	CountLastMonth int         `db:"count_last_month"`
	CountPrevMonth int         `db:"count_prev_month"`
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

	err = a.Usage.BySite(ctx, id)
	if err != nil {
		return err
	}

	err = zdb.MustGet(ctx).GetContext(ctx, a, `
		select
			(select created_at from hits where site=$1 order by created_at desc limit 1) as last_data,
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

// ByCode gets stats for a single site.
func (a *AdminSiteStat) ByCode(ctx context.Context, code string) error {
	err := a.Site.ByHost(ctx, code+"."+cfg.Domain)
	if err != nil {
		return err
	}
	return a.ByID(ctx, a.Site.ID)
}

type AdminUsage struct {
	Site   int64  `db:"site"`
	Domain string `db:"domain"`
	Count  int    `db:"count"`
}

type AdminUsages []AdminUsage

func (a *AdminUsages) List(ctx context.Context) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, a, `
		select site, domain, sum(count) as count from usage
		where vetted=0
		group by site, domain
		having sum(count)>5000
		order by count desc`),
		"AdminUsage")
}

// BySite gets usage for one site.
func (a *AdminUsages) BySite(ctx context.Context, id int64) error {
	return errors.Wrap(zdb.MustGet(ctx).SelectContext(ctx, a, `
		select site, domain, sum(count) as count from usage
		where site=$1
		group by site, domain
		order by count desc`, id),
		"AdminUsage")
}

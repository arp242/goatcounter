// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/errors"
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

	ival := interval(30)
	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			sites.id,
			sites.parent,
			sites.code,
			sites.name,
			sites.created_at,
			(case when substr(sites.stripe, 0, 9) = 'cus_free' then 'free' else sites.plan end) as plan,
			sites.link_domain,
			users.name as user,
			users.email,
			count(*) - 1 as count
		from sites
		left join hits on hits.site=sites.id
		left join users on users.site=coalesce(sites.parent, sites.id)
		where hits.created_at >= %s
		group by sites.id, sites.code, sites.name, sites.created_at, users.name, users.email, plan
		having count(*) > 1000
		order by %s desc`, ival, order))
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
	Site           Site      `db:"-"`
	User           User      `db:"-"`
	LastData       time.Time `db:"last_data"`
	CountTotal     int       `db:"count_total"`
	CountLastMonth int       `db:"count_last_month"`
	CountPrevMonth int       `db:"count_prev_month"`
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

	ival30 := interval(30)
	ival60 := interval(30)
	err = zdb.MustGet(ctx).GetContext(ctx, a, fmt.Sprintf(`
		select
			coalesce((select created_at from hits where site=$1 order by created_at desc limit 1), '1970-01-01') as last_data,
			(select count(*) from hits where site=$1) as count_total,
			(select count(*) from hits where site=$1
				and created_at >= %[1]s) as count_last_month,
			(select count(*) from hits where site=$1
				and created_at >= %[2]s
				and created_at <= %[1]s
			) as count_prev_month
		`, ival30, ival60), id)
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

type AdminPgStats []struct {
	Total    float64 `db:"total"`
	MeanTime float64 `db:"mean_time"`
	Calls    int     `db:"calls"`
	QueryID  int64   `db:"queryid"`
	Query    string  `db:"query"`
}

func (a *AdminPgStats) List(ctx context.Context, order string) error {
	if order == "" {
		order = "total"
	}
	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			(total_time / 1000 / 60) as total,
			mean_time,
			calls,
			queryid,
			query
		from pg_stat_statements where
			userid = (select usesysid from pg_user where usename = CURRENT_USER) and
			calls > 20 and
			query !~* '^ *(copy|create|alter|explain) '
		order by %s desc
		limit 100
	`, order))
	if err != nil {
		return fmt.Errorf("AdminPgStats.List: %w", err)
	}

	// Normalize the indent a bit, because there are often of extra tabs inside
	// Go literal strings.
	aa := *a
	for i := range aa {
		lines := strings.Split(aa[i].Query, "\n")
		if len(lines) < 2 {
			continue
		}

		n := strings.Count(lines[1], "\t") - 1
		for j := range lines {
			lines[j] = strings.Replace(lines[j], "\t", "", n)
		}

		aa[i].Query = strings.Join(lines, "\n")
	}

	*a = aa
	return nil
}

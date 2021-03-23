// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/zstring"
)

type BosmangStat struct {
	ID            int64     `db:"site_id"`
	Parent        *int64    `db:"parent"`
	Code          string    `db:"code"`
	Stripe        *string   `db:"stripe"`
	BillingAmount *string   `db:"billing_amount"`
	Email         string    `db:"email"`
	CreatedAt     time.Time `db:"created_at"`
	Plan          string    `db:"plan"`
	LastMonth     int       `db:"last_month"`
	Total         int       `db:"total"`
}

type BosmangStats []BosmangStat

// List stats for all sites, for all time.
func (a *BosmangStats) List(ctx context.Context) error {
	err := zdb.Select(ctx, a, "load:bosmang.List", zdb.DumpExplain)
	if err != nil {
		return errors.Wrap(err, "BosmangStats.List")
	}

	// Add all the child plan counts to the parents.
	type x struct {
		total, last_month int
		code              string
	}
	ch := make(map[int64][]x)
	aa := *a
	for _, s := range aa {
		if s.Parent == nil {
			continue
		}
		ch[*s.Parent] = append(ch[*s.Parent], x{code: s.Code, total: s.Total, last_month: s.LastMonth})
	}

	filter := make(BosmangStats, 0, len(aa))
	curr := strings.NewReplacer("EUR ", "€", "USD ", "$")
	for _, s := range aa {
		c, ok := ch[s.ID]
		if !ok {
			continue
		}

		for _, cc := range c {
			s.Total += cc.total
			s.LastMonth += cc.last_month
			s.Code += " | " + cc.code
		}

		if s.BillingAmount != nil {
			s.BillingAmount = zstring.NewPtr(curr.Replace(*s.BillingAmount)).P
		}
		filter = append(filter, s)
	}

	sort.Slice(filter, func(i, j int) bool { return filter[i].LastMonth > filter[j].LastMonth })
	*a = filter
	return nil
}

type BosmangSiteStat struct {
	MainSite Site  `db:"-"`
	Sites    Sites `db:"-"`
	Users    Users `db:"-"`

	Stats []struct {
		Code           string    `db:"code"`
		LastData       time.Time `db:"last_data"`
		CountTotal     int       `db:"count_total"`
		CountLastMonth int       `db:"count_last_month"`
		CountPrevMonth int       `db:"count_prev_month"`
	}
}

// ByID gets stats for a single site.
func (a *BosmangSiteStat) ByID(ctx context.Context, id int64) error {
	err := a.MainSite.ByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}
	err = a.MainSite.GetMain(ctx)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}
	err = a.Sites.ForThisAccount(WithSite(ctx, &a.MainSite), false)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}
	err = a.Users.List(ctx, id)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}

	var (
		ival30 = interval(ctx, 30)
		ival60 = interval(ctx, 60)
		query  []string
	)
	for _, s := range a.Sites {
		query = append(query, fmt.Sprintf(`
			select
				(select code from sites where site_id=%[1]d),

				coalesce((
					select hour from hit_counts where site_id=%[1]d order by hour desc limit 1),
				'1970-01-01') as last_data,

				coalesce((
					select sum(total) from hit_counts where site_id=%[1]d),
				0) as count_total,

				coalesce((
					select sum(total) from hit_counts where site_id=%[1]d
					and hour >= %[2]s),
				0) as count_last_month,

				coalesce((
					select sum(total) from hit_counts where site_id=%[1]d and hour >= %[3]s and hour <= %[2]s
				), 0) as count_prev_month
			`, s.ID, ival30, ival60))
	}

	err = zdb.Select(ctx, &a.Stats,
		"/* BosmangSiteStat.ByID */\n"+strings.Join(query, "union\n")+"\norder by count_total desc")
	return errors.Wrap(err, "BosmangSiteStats.ByID")
}

// Find gets stats for a single site.
func (a *BosmangSiteStat) Find(ctx context.Context, ident string) error {
	id, err := strconv.ParseInt(ident, 10, 64)
	switch {
	case id > 0:
		// Do nothing
	case strings.ContainsRune(ident, '@'):
		var u User
		err = u.ByEmail(ctx, ident)
		id = u.Site
	default:
		var s Site
		err = s.ByCode(ctx, ident)
		id = s.ID
	}
	if err != nil {
		return err
	}

	return a.ByID(ctx, id)
}

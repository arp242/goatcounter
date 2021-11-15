// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zcache"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zstring"
)

type BosmangStat struct {
	ID            int64     `db:"site_id"`
	Codes         string    `db:"codes"`
	Stripe        *string   `db:"stripe"`
	BillingAmount *string   `db:"billing_amount"`
	Email         string    `db:"email"`
	CreatedAt     time.Time `db:"created_at"`
	Plan          string    `db:"plan"`
	LastMonth     int       `db:"last_month"`
	Total         int       `db:"total"`
	Avg           int       `db:"avg"`
}

type BosmangStats []BosmangStat

// List stats for all sites, for all time.
func (a *BosmangStats) List(ctx context.Context) error {
	err := zdb.Select(ctx, a, "load:bosmang.List")
	if err != nil {
		return errors.Wrap(err, "BosmangStats.List")
	}

	curr := strings.NewReplacer("EUR ", "€", "USD ", "$")
	aa := *a
	for i := range aa {
		if aa[i].BillingAmount != nil {
			aa[i].BillingAmount = zstring.NewPtr(curr.Replace(*aa[i].BillingAmount)).P
		}
	}
	return nil
}

type BosmangSiteStat struct {
	Account Site
	Sites   Sites
	Users   Users
	Usage   AccountUsage
}

// ByID gets stats for a single site.
func (a *BosmangSiteStat) ByID(ctx context.Context, id int64) error {
	var s Site
	err := s.ByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}

	acc, err := GetAccount(WithSite(ctx, &s))
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}
	a.Account = *acc
	err = a.Sites.ForThisAccount(WithSite(ctx, &a.Account), false)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}
	err = a.Users.List(ctx, id)
	if err != nil {
		return errors.Wrap(err, "BosmangSiteStats.ByID")
	}

	err = a.Usage.Get(WithSite(ctx, &a.Account))
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
		err = zdb.Get(ctx, &u, `select * from users where lower(email) = lower(?)`, ident)
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

func ListCache(ctx context.Context) map[string]struct {
	Size  int64
	Items map[string]string
} {
	c := make(map[string]struct {
		Size  int64
		Items map[string]string
	})

	caches := map[string]func(context.Context) *zcache.Cache{
		"sites":          cacheSites,
		"ua":             cacheUA,
		"browsers":       cacheBrowsers,
		"systems":        cacheSystems,
		"paths":          cachePaths,
		"loc":            cacheLoc,
		"changed_titles": cacheChangedTitles,
	}

	for name, f := range caches {
		var (
			content = f(ctx).Items()
			s       = zruntime.SizeOf(content)
			items   = make(map[string]string)
		)
		for k, v := range content {
			items[k] = fmt.Sprintf("%s\n", zjson.MustMarshalIndent(v.Object, "", "  "))
			s += c[name].Size + zruntime.SizeOf(v.Object)
		}
		c[name] = struct {
			Size  int64
			Items map[string]string
		}{s / 1024, items}
	}

	{
		var (
			name    = "sites_host"
			content = cacheSitesHost(ctx).Items()
			s       = zruntime.SizeOf(content)
			items   = make(map[string]string)
		)
		for k, v := range content {
			items[k] = v
			s += c[name].Size + zruntime.SizeOf(v)
		}
		c[name] = struct {
			Size  int64
			Items map[string]string
		}{s / 1024, items}
	}
	return c
}

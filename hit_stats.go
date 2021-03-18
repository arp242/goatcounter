// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

type HitStat struct {
	ID          string  `db:"id"`
	Name        string  `db:"name"`
	Count       int     `db:"count"`
	CountUnique int     `db:"count_unique"`
	RefScheme   *string `db:"ref_scheme"`
}

type HitStats struct {
	More  bool
	Stats []HitStat
}

func asUTCDate(u *User, t time.Time) string {
	return t.In(u.Settings.Timezone.Location).Format("2006-01-02")
}

// ByRef lists all paths by referrer.
func (h *HitStats) ByRef(ctx context.Context, start, end time.Time, pathFilter []int64, ref string) error {
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ByRef", zdb.P{
		"site":   MustGetSite(ctx).ID,
		"start":  start,
		"end":    end,
		"filter": pathFilter,
		"ref":    ref,
	})
	return errors.Wrap(err, "HitStats.ByRef")
}

// ListBrowsers lists all browser statistics for the given time period.
func (h *HitStats) ListBrowsers(ctx context.Context, start, end time.Time, pathFilter []int64, limit, offset int) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListBrowsers", zdb.P{
		"site":   site.ID,
		"start":  asUTCDate(user, start),
		"end":    asUTCDate(user, end),
		"filter": pathFilter,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListBrowsers")
}

// ListBrowser lists all the versions for one browser.
func (h *HitStats) ListBrowser(ctx context.Context, browser string, start, end time.Time, pathFilter []int64) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListBrowser", zdb.P{
		"site":    site.ID,
		"start":   asUTCDate(user, start),
		"end":     asUTCDate(user, end),
		"filter":  pathFilter,
		"browser": browser,
	})
	return errors.Wrap(err, "HitStats.ListBrowser")
}

// ListSystems lists OS statistics for the given time period.
func (h *HitStats) ListSystems(ctx context.Context, start, end time.Time, pathFilter []int64, limit, offset int) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSystems", zdb.P{
		"site":   site.ID,
		"start":  asUTCDate(user, start),
		"end":    asUTCDate(user, end),
		"filter": pathFilter,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListSystems")
}

// ListSystem lists all the versions for one system.
func (h *HitStats) ListSystem(ctx context.Context, system string, start, end time.Time, pathFilter []int64) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSystem", zdb.P{
		"site":   site.ID,
		"start":  asUTCDate(user, start),
		"end":    asUTCDate(user, end),
		"filter": pathFilter,
		"system": system,
	})
	return errors.Wrap(err, "HitStats.ListSystem")
}

const (
	sizePhones      = "Phones"
	sizeLargePhones = "Large phones, small tablets"
	sizeTablets     = "Tablets and small laptops"
	sizeDesktop     = "Computer monitors"
	sizeDesktopHD   = "Computer monitors larger than HD"
	sizeUnknown     = "(unknown)"
)

// ListSizes lists all device sizes.
func (h *HitStats) ListSizes(ctx context.Context, start, end time.Time, pathFilter []int64) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSizes", zdb.P{
		"site":   site.ID,
		"start":  asUTCDate(user, start),
		"end":    asUTCDate(user, end),
		"filter": pathFilter,
	})
	if err != nil {
		return errors.Wrap(err, "HitStats.ListSize")
	}

	// Group a bit more user-friendly.
	ns := []HitStat{
		{Name: sizePhones, Count: 0, CountUnique: 0},
		{Name: sizeLargePhones, Count: 0, CountUnique: 0},
		{Name: sizeTablets, Count: 0, CountUnique: 0},
		{Name: sizeDesktop, Count: 0, CountUnique: 0},
		{Name: sizeDesktopHD, Count: 0, CountUnique: 0},
		{Name: sizeUnknown, Count: 0, CountUnique: 0},
	}

	for i := range h.Stats {
		x, _ := strconv.ParseInt(h.Stats[i].Name, 10, 16)
		switch {
		case x == 0:
			ns[5].Count += h.Stats[i].Count
			ns[5].CountUnique += h.Stats[i].CountUnique
		case x <= 384:
			ns[0].Count += h.Stats[i].Count
			ns[0].CountUnique += h.Stats[i].CountUnique
		case x <= 1024:
			ns[1].Count += h.Stats[i].Count
			ns[1].CountUnique += h.Stats[i].CountUnique
		case x <= 1440:
			ns[2].Count += h.Stats[i].Count
			ns[2].CountUnique += h.Stats[i].CountUnique
		case x <= 1920:
			ns[3].Count += h.Stats[i].Count
			ns[3].CountUnique += h.Stats[i].CountUnique
		default:
			ns[4].Count += h.Stats[i].Count
			ns[4].CountUnique += h.Stats[i].CountUnique
		}
	}
	h.Stats = ns

	return nil
}

// ListSize lists all sizes for one grouping.
func (h *HitStats) ListSize(ctx context.Context, name string, start, end time.Time, pathFilter []int64) error {
	var (
		min_size, max_size int
		empty              bool
	)
	switch name {
	case sizePhones:
		max_size = 384
	case sizeLargePhones:
		min_size, max_size = 384, 1024
	case sizeTablets:
		min_size, max_size = 1024, 1440
	case sizeDesktop:
		min_size, max_size = 1440, 1920
	case sizeDesktopHD:
		min_size, max_size = 1920, 99999
	case sizeUnknown:
		empty = true
	default:
		return errors.Errorf("HitStats.ListSizes: invalid value for name: %#v", name)
	}

	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSize", zdb.P{
		"site":     site.ID,
		"start":    asUTCDate(user, start),
		"end":      asUTCDate(user, end),
		"filter":   pathFilter,
		"min_size": min_size,
		"max_size": max_size,
		"empty":    empty,
	})
	if err != nil {
		return errors.Wrap(err, "HitStats.ListSize")
	}
	for i := range h.Stats { // TODO: see if we can do this in SQL.
		h.Stats[i].Name = strings.ReplaceAll(h.Stats[i].Name, "↔", "↔\ufe0e")
	}
	return nil
}

// ListLocations lists all location statistics for the given time period.
func (h *HitStats) ListLocations(ctx context.Context, start, end time.Time, pathFilter []int64, limit, offset int) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListLocations", zdb.P{
		"site":   site.ID,
		"start":  asUTCDate(user, start),
		"end":    asUTCDate(user, end),
		"filter": pathFilter,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListLocations")
}

// ListLocation lists all divisions for a location
func (h *HitStats) ListLocation(ctx context.Context, country string, start, end time.Time, pathFilter []int64) error {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListLocation", zdb.P{
		"site":    site.ID,
		"start":   asUTCDate(user, start),
		"end":     asUTCDate(user, end),
		"filter":  pathFilter,
		"country": country,
	})
	return errors.Wrap(err, "HitStats.ListLocation")
}

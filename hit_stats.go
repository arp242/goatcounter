// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

// ListBrowsers lists all browser statistics for the given time period.
func (h *Stats) ListBrowsers(ctx context.Context, start, end time.Time, limit, offset int) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `/* Stats.ListBrowsers */
		select
			browser as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from browser_stats
		where site=$1 and day>=$2 and day<=$3
		group by browser
		order by count_unique desc, name asc
		limit $4 offset $5
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), limit+1, offset)

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "Stats.ListBrowsers browsers")
}

// ListBrowser lists all the versions for one browser.
func (h *Stats) ListBrowser(ctx context.Context, browser string, start, end time.Time) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `
		select
			browser || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from browser_stats
		where site=$1 and day>=$2 and day<=$3 and lower(browser)=lower($4)
		group by browser, version
		order by count_unique desc, name asc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), browser)
	return errors.Wrap(err, "Stats.ListBrowser")
}

// ListSystems lists OS statistics for the given time period.
func (h *Stats) ListSystems(ctx context.Context, start, end time.Time, limit, offset int) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `/* Stats.ListSystem */
		select
			system as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		where site=$1 and day>=$2 and day<=$3
		group by system
		order by count_unique desc, name asc
		limit $4 offset $5
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), limit+1, offset)

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "Stats.ListSystems")
}

// ListSystem lists all the versions for one system.
func (h *Stats) ListSystem(ctx context.Context, system string, start, end time.Time) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `
		select
			system || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		where site=$1 and day >= $2 and day <= $3 and lower(system)=lower($4)
		group by system, version
		order by count_unique desc, name asc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), system)
	return errors.Wrap(err, "Stats.ListSystem")
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
func (h *Stats) ListSizes(ctx context.Context, start, end time.Time) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `/* Stats.ListSizes */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from size_stats
		where site=$1 and day >= $2 and day <= $3
		group by width
		order by count_unique desc, name asc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "Stats.ListSize")
	}

	// Group a bit more user-friendly.
	ns := []StatT{
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
func (h *Stats) ListSize(ctx context.Context, name string, start, end time.Time) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	var where string
	switch name {
	case sizePhones:
		where = "width != 0 and width <= 384"
	case sizeLargePhones:
		where = "width != 0 and width <= 1024 and width > 384"
	case sizeTablets:
		where = "width != 0 and width <= 1440 and width > 1024"
	case sizeDesktop:
		where = "width != 0 and width <= 1920 and width > 1440"
	case sizeDesktopHD:
		where = "width != 0 and width > 1920"
	case sizeUnknown:
		where = "width = 0"
	default:
		return errors.Errorf("Stats.ListSizes: invalid value for name: %#v", name)
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, fmt.Sprintf(`/* Stats.ListSize */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from size_stats
		where
			site=$1 and day >= $2 and day <= $3 and
			%s
		group by width
	`, where), MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "Stats.ListSize")
	}

	grouped := make(map[string]int)
	groupedUnique := make(map[string]int)
	for i := range h.Stats {
		grouped[fmt.Sprintf("\ufe0e↔ %spx", h.Stats[i].Name)] += h.Stats[i].Count
		groupedUnique[fmt.Sprintf("\ufe0e↔ %spx", h.Stats[i].Name)] += h.Stats[i].CountUnique
	}

	ns := make([]StatT, len(grouped))
	i := 0
	for width, count := range grouped {
		ns[i] = StatT{
			Name:        width,
			Count:       count,
			CountUnique: groupedUnique[width],
		}
		i++
	}
	sort.Slice(ns, func(i int, j int) bool { return ns[i].Count > ns[j].Count })
	h.Stats = ns

	return nil
}

// ListLocations lists all location statistics for the given time period.
func (h *Stats) ListLocations(ctx context.Context, start, end time.Time, limit, offset int) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.MustGet(ctx).SelectContext(ctx, &h.Stats, `/* Stats.ListLocations */
		select
			iso_3166_1.name as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from location_stats
		join iso_3166_1 on iso_3166_1.alpha2=location
		where site=$1 and day >= $2 and day <= $3
		group by location, iso_3166_1.name
		order by count_unique desc, name asc
		limit $4 offset $5
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), limit+1, offset)

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "Stats.ListLocations")
}

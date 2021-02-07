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
func (h *Stats) ListBrowsers(ctx context.Context, start, end time.Time, pathFilter []int64, limit, offset int) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListBrowsers */
		with x as (
			select
				browser_id,
				sum(count) as count,
				sum(count_unique) as count_unique
			from browser_stats
			where
				site_id = :site and day >= :start and day <= :end
				{{:filter and path_id in (:filter)}}
			group by browser_id
			order by count_unique desc
		)
		select
			browsers.name,
			sum(x.count) as count,
			sum(x.count_unique) as count_unique
		from x
		join browsers using (browser_id)
		group by browsers.name
		order by count_unique desc
		limit :limit offset :offset`,
		struct {
			Site   int64
			Start  string
			End    string
			Filter []int64
			Limit  int
			Offset int
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter, limit + 1, offset})

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "Stats.ListBrowsers browsers")
}

// ListBrowser lists all the versions for one browser.
func (h *Stats) ListBrowser(ctx context.Context, browser string, start, end time.Time, pathFilter []int64) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListBrowser */
		select
			name || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from browser_stats
		join browsers using (browser_id)
		where
			site_id = :site and day >= :start and day <= :end and
			{{:filter path_id in (:filter) and}}
			lower(name) = lower(:browser)
		group by name, version
		order by count_unique desc, name asc `,
		struct {
			Site    int64
			Start   string
			End     string
			Filter  []int64
			Browser string
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter, browser})
	return errors.Wrap(err, "Stats.ListBrowser")
}

// ListSystems lists OS statistics for the given time period.
func (h *Stats) ListSystems(ctx context.Context, start, end time.Time, pathFilter []int64, limit, offset int) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListSystem */
		with x as (
			select
				system_id,
				sum(count) as count,
				sum(count_unique) as count_unique
			from system_stats
			where
				site_id = :site and day >= :start and day <= :end
				{{:filter and path_id in (:filter)}}
			group by system_id
			order by count_unique desc
		)
		select
			systems.name,
			sum(x.count) as count,
			sum(x.count_unique) as count_unique
		from x
		join systems using (system_id)
		group by systems.name
		order by count_unique desc
		limit :limit offset :offset`,
		struct {
			Site   int64
			Start  string
			End    string
			Filter []int64
			Limit  int
			Offset int
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter, limit + 1, offset})

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "Stats.ListSystems")
}

// ListSystem lists all the versions for one system.
func (h *Stats) ListSystem(ctx context.Context, system string, start, end time.Time, pathFilter []int64) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListSystem */
		select
			name || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		join systems using (system_id)
		where
			site_id = :site and day >= :start and day <= :end and
			{{:filter path_id in (:filter) and}}
			lower(name) = lower(:system)
		group by name, version
		order by count_unique desc, name asc`,
		struct {
			Site   int64
			Start  string
			End    string
			Filter []int64
			System string
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter, system})
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
func (h *Stats) ListSizes(ctx context.Context, start, end time.Time, pathFilter []int64) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListSizes */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from size_stats
		where
			site_id = :site and day >= :start and day <= :end
			{{:filter and path_id in (:filter)}}
		group by width
		order by count_unique desc, name asc`,
		struct {
			Site   int64
			Start  string
			End    string
			Filter []int64
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter})
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
func (h *Stats) ListSize(ctx context.Context, name string, start, end time.Time, pathFilter []int64) error {
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

	// TODO: where can be paramters
	err := zdb.Select(ctx, &h.Stats, fmt.Sprintf(`/* Stats.ListSize */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from size_stats
		where
			site_id = :site and day >= :start and day <= :end
			{{:filter and path_id in (:filter)}}
			and %s
		group by width`, where),
		struct {
			Site   int64
			Start  string
			End    string
			Filter []int64
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter})
	if err != nil {
		return errors.Wrap(err, "Stats.ListSize")
	}

	grouped := make(map[string]int)
	groupedUnique := make(map[string]int)
	for i := range h.Stats {
		grouped[fmt.Sprintf("↔\ufe0e %spx", h.Stats[i].Name)] += h.Stats[i].Count
		groupedUnique[fmt.Sprintf("↔\ufe0e %spx", h.Stats[i].Name)] += h.Stats[i].CountUnique
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
func (h *Stats) ListLocations(ctx context.Context, start, end time.Time, pathFilter []int64, limit, offset int) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListLocations */
		with x as (
			select
				substr(location, 0, 3) as loc,
				sum(count)             as count,
				sum(count_unique)      as count_unique
			from location_stats
			where
				site_id = :site and day >= :start and day <= :end
				{{:filter and path_id in (:filter)}}
			group by loc
			order by count_unique desc, loc
			limit :limit offset :offset
		)
		select
			locations.iso_3166_2   as id,
			locations.country_name as name,
			x.count                as count,
			x.count_unique         as count_unique
		from x
		join locations on locations.iso_3166_2 = x.loc
		order by count_unique desc, name asc`,
		struct {
			Site   int64
			Start  string
			End    string
			Filter []int64
			Limit  int
			Offset int
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter, limit + 1, offset})

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "Stats.ListLocations")
}

// ListLocation lists all divisions for a location
func (h *Stats) ListLocation(ctx context.Context, country string, start, end time.Time, pathFilter []int64) error {
	start = start.In(MustGetSite(ctx).Settings.Timezone.Location)
	end = end.In(MustGetSite(ctx).Settings.Timezone.Location)

	err := zdb.Select(ctx, &h.Stats, `/* Stats.ListLocation */
		select
			coalesce(region_name, '(unknown)') as name,
			sum(count)                         as count,
			sum(count_unique)                  as count_unique
		from location_stats
		join locations on location = iso_3166_2
		where
			site_id = :site and day >= :start and day <= :end and
			{{:filter path_id in (:filter) and}}
			country = :country
		group by iso_3166_2, name
		order by count_unique desc, name asc`,
		struct {
			Site    int64
			Start   string
			End     string
			Filter  []int64
			Country string
		}{MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), pathFilter, country})
	return errors.Wrap(err, "Stats.ListLocation")
}

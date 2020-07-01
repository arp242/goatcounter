// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

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
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListSizes */
		select
			width as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from size_stats
		where site=$1 and day >= $2 and day <= $3
		group by width
		order by count_unique desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return errors.Wrap(err, "Stats.ListSize")
	}

	// Group a bit more user-friendly.
	// TODO: ideally I'd like to make a line chart in the future, in which case
	// this should no longer be needed.
	ns := Stats{
		{Name: sizePhones, Count: 0, CountUnique: 0},
		{Name: sizeLargePhones, Count: 0, CountUnique: 0},
		{Name: sizeTablets, Count: 0, CountUnique: 0},
		{Name: sizeDesktop, Count: 0, CountUnique: 0},
		{Name: sizeDesktopHD, Count: 0, CountUnique: 0},
		{Name: sizeUnknown, Count: 0, CountUnique: 0},
	}

	hh := *h
	for i := range hh {
		x, _ := strconv.ParseInt(hh[i].Name, 10, 16)
		switch {
		case x == 0:
			ns[5].Count += hh[i].Count
			ns[5].CountUnique += hh[i].CountUnique
		case x <= 384:
			ns[0].Count += hh[i].Count
			ns[0].CountUnique += hh[i].CountUnique
		case x <= 1024:
			ns[1].Count += hh[i].Count
			ns[1].CountUnique += hh[i].CountUnique
		case x <= 1440:
			ns[2].Count += hh[i].Count
			ns[2].CountUnique += hh[i].CountUnique
		case x <= 1920:
			ns[3].Count += hh[i].Count
			ns[3].CountUnique += hh[i].CountUnique
		default:
			ns[4].Count += hh[i].Count
			ns[4].CountUnique += hh[i].CountUnique
		}
	}
	*h = ns

	return nil
}

// ListSize lists all sizes for one grouping.
func (h *Stats) ListSize(ctx context.Context, name string, start, end time.Time) (int, error) {
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
		return 0, errors.Errorf("Stats.ListSizes: invalid value for name: %#v", name)
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, h, fmt.Sprintf(`/* Stats.ListLocations */
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
		return 0, errors.Wrap(err, "Stats.ListSize")
	}

	grouped := make(map[string]int)
	groupedUnique := make(map[string]int)
	hh := *h
	for i := range hh {
		grouped[fmt.Sprintf("↔ %spx", hh[i].Name)] += hh[i].Count
		groupedUnique[fmt.Sprintf("↔ %spx", hh[i].Name)] += hh[i].CountUnique
	}

	ns := Stats{}
	total := 0
	for width, count := range grouped {
		total += count
		ns = append(ns, StatT{
			Name:        width,
			Count:       count,
			CountUnique: groupedUnique[width],
		})
	}
	sort.Slice(ns, func(i int, j int) bool { return ns[i].Count > ns[j].Count })
	*h = ns

	return total, nil
}

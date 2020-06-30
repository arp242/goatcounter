// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

// ListSystems lists OS statistics for the given time period.
func (h *Stats) ListSystems(ctx context.Context, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListSystem */
		select
			system as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		where site=$1 and day>=$2 and day<=$3
		group by system
		order by count_unique desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "Stats.OS")
	}

	var total int
	for _, b := range *h {
		total += b.CountUnique
	}

	return total, nil
}

// ListSystem lists all the versions for one system.
func (h *Stats) ListSystem(ctx context.Context, system string, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			system || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from system_stats
		where site=$1 and day >= $2 and day <= $3 and lower(system)=lower($4)
		group by system, version
		order by count_unique desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), system)
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListSystem")
	}

	var total int
	for _, b := range *h {
		total += b.Count
	}
	return total, nil
}

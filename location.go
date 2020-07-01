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

// ListLocations lists all location statistics for the given time period.
func (h *Stats) ListLocations(ctx context.Context, start, end time.Time) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListLocations */
		select
			iso_3166_1.name as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from location_stats
		join iso_3166_1 on iso_3166_1.alpha2=location
		where site=$1 and day >= $2 and day <= $3
		group by location, iso_3166_1.name
		order by count_unique desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))

	return errors.Wrap(err, "Stats.ListLocations")
}

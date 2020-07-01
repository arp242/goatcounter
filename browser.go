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

// ListBrowsers lists all browser statistics for the given time period.
func (h *Stats) ListBrowsers(ctx context.Context, start, end time.Time) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListBrowsers */
		select
			browser as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from browser_stats
		where site=$1 and day >= $2 and day <= $3
		group by browser
		order by count_unique desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"))

	return errors.Wrap(err, "Stats.ListBrowsers browsers")
}

// ListBrowser lists all the versions for one browser.
func (h *Stats) ListBrowser(ctx context.Context, browser string, start, end time.Time) (int, error) {
	err := zdb.MustGet(ctx).SelectContext(ctx, h, `
		select
			browser || ' ' || version as name,
			sum(count) as count,
			sum(count_unique) as count_unique
		from browser_stats
		where site=$1 and day >= $2 and day <= $3 and lower(browser)=lower($4)
		group by browser, version
		order by count_unique desc
	`, MustGetSite(ctx).ID, start.Format("2006-01-02"), end.Format("2006-01-02"), browser)
	if err != nil {
		return 0, errors.Wrap(err, "Stats.ListBrowser")
	}

	var total int
	for _, b := range *h {
		total += b.CountUnique
	}
	return total, nil
}

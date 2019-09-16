// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

type BrowserStats []struct {
	Browser string
	Count   int
}

func (h *BrowserStats) List(ctx context.Context, start, end time.Time) (uint64, error) {
	site := MustGetSite(ctx)
	err := MustGetDB(ctx).SelectContext(ctx, h, `
		select browser, sum(count) as count from browser_stats
		where site=$1 and day >= $2 and day <= $3
		group by browser
		order by count desc
	`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return 0, errors.Wrap(err, "BrowserStats.List")
	}

	var total uint64
	for _, b := range *h {
		total += uint64(b.Count)
	}
	return total, nil
}

// ListBrowser lists all the versions for one browser.
func (h *BrowserStats) ListBrowser(ctx context.Context, browser string, start, end time.Time) (uint64, error) {
	site := MustGetSite(ctx)
	err := MustGetDB(ctx).SelectContext(ctx, h, `
		select
			version as browser,
			sum(count) as count
		from browser_stats
		where site=$1 and day >= $2 and day <= $3 and lower(browser)=lower($4)
		group by browser, version
		order by count desc
	`, site.ID, start.Format("2006-01-02"), end.Format("2006-01-02"), browser)
	if err != nil {
		return 0, errors.Wrap(err, "BrowserStats.ListBrowser")
	}

	var total uint64
	for _, b := range *h {
		total += uint64(b.Count)
	}
	return total, nil
}

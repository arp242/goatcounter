// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

// ListRefsByPath lists all references for a path.
func (h *Stats) ListRefsByPath(ctx context.Context, path string, start, end time.Time, offset int) (bool, error) {
	site := MustGetSite(ctx)

	limit := site.Settings.Limits.Ref
	if limit == 0 {
		limit = 10
	}

	fmt.Println(zdb.ApplyPlaceholders(`/* Stats.ListRefsByPath */
		select
			coalesce(sum(total), 0) as count,
			coalesce(sum(total_unique), 0) as count_unique,
			max(ref_scheme) as ref_scheme,
			ref as name
		from ref_counts
		where
			site=$1 and
			lower(path)=lower($2) and
			hour>=$3 and
			hour<=$4
		group by ref
		order by count_unique desc, ref desc
		limit $5 offset $6`,
		site.ID, path, start.Format(zdb.Date), end.Format(zdb.Date), limit+1, offset))

	err := zdb.MustGet(ctx).SelectContext(ctx, h, `/* Stats.ListRefsByPath */
		select
			coalesce(sum(total), 0) as count,
			coalesce(sum(total_unique), 0) as count_unique,
			max(ref_scheme) as ref_scheme,
			ref as name
		from ref_counts
		where
			site=$1 and
			lower(path)=lower($2) and
			hour>=$3 and
			hour<=$4
		group by ref
		order by count_unique desc, ref desc
		limit $5 offset $6`,
		site.ID, path, start.Format(zdb.Date), end.Format(zdb.Date), limit+1, offset)
	if err != nil {
		errors.Wrap(err, "Stats.ListRefsByPath")
	}

	var more bool
	if len(*h) > limit {
		more = true
		hh := *h
		hh = hh[:len(hh)-1]
		*h = hh
	}

	return more, nil
}

// ListTopRefs lists all ref statistics for the given time period, excluding
// referrals from the configured LinkDomain.
//
// The returned count is the count without LinkDomain, and is different from the
// total number of hits.
//
// TODO: after ref_counts it no longer lists "unknown".
func (h *Stats) ListTopRefs(ctx context.Context, start, end time.Time, offset int) (bool, error) {
	site := MustGetSite(ctx)

	limit := site.Settings.Limits.Ref
	if limit == 0 {
		limit = 10
	}

	where := ` where site=? and hour>=? and hour<=?`
	args := []interface{}{site.ID, start.Format(zdb.Date), end.Format(zdb.Date)}
	if site.LinkDomain != "" {
		where += " and ref not like ? "
		args = append(args, site.LinkDomain+"%")
	}

	db := zdb.MustGet(ctx)
	err := db.SelectContext(ctx, h, db.Rebind(`/* Stats.ListTopRefs */
		select
			coalesce(sum(total), 0) as count,
			coalesce(sum(total_unique), 0) as count_unique,
			max(ref_scheme) as ref_scheme,
			ref as name
		from ref_counts`+
		where+`
		group by ref
		order by count_unique desc
		limit ? offset ?`), append(args, limit+1, offset)...)
	if err != nil {
		return false, errors.Wrap(err, "Stats.ListAllRefs")
	}

	var more bool
	if len(*h) > limit {
		more = true
		hh := *h
		hh = hh[:len(hh)-1]
		*h = hh
	}

	return more, nil
}

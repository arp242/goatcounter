// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"strconv"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/ztime"
)

type HitStat struct {
	// ID for selecting more details; not present in the detail view.
	ID    string `db:"id" json:"id,omitempty"`
	Name  string `db:"name" json:"name"`   // Display name.
	Count int    `db:"count" json:"count"` // Number of visitors.

	// What kind of referral this is; only set when retrieving referrals {enum: h g c o}.
	//
	//  h   HTTP Referal header.
	//  g   Generated; for example are Google domains (google.com, google.nl,
	//      google.co.nz, etc.) are grouped as the generated referral "Google".
	//  c   Campaign (via query parameter)
	//  o   Other
	RefScheme *string `db:"ref_scheme" json:"ref_scheme,omitempty"`
}

type HitStats struct {
	More  bool      `json:"more"`
	Stats []HitStat `json:"stats"`
}

func asUTCDate(u *User, t time.Time) string {
	return t.In(u.Settings.Timezone.Location).Format("2006-01-02")
}

// ListTopRefs lists all ref statistics for the given time period, excluding
// referrals from the configured LinkDomain.
//
// The returned count is the count without LinkDomain, and is different from the
// total number of hits.
func (h *HitStats) ListTopRefs(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	site := MustGetSite(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:ref.ListTopRefs.sql", map[string]any{
		"site":       site.ID,
		"start":      rng.Start,
		"end":        rng.End,
		"filter":     pathFilter,
		"ref":        site.LinkDomainURL(false) + "%",
		"limit":      limit + 1,
		"limit2":     limit + (limit * 3),
		"offset":     offset,
		"has_domain": site.LinkDomain != "",
	})
	if err != nil {
		return errors.Wrap(err, "HitStats.ListAllRefs")
	}

	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return nil
}

// ListTopRef lists all paths by referrer.
func (h *HitStats) ListTopRef(ctx context.Context, ref string, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ByRef", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  rng.Start,
		"end":    rng.End,
		"filter": pathFilter,
		"ref":    ref,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ByRef")
}

// ListBrowsers lists all browser statistics for the given time period.
func (h *HitStats) ListBrowsers(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListBrowsers", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
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
func (h *HitStats) ListBrowser(ctx context.Context, browser string, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListBrowser", map[string]any{
		"site":    MustGetSite(ctx).ID,
		"start":   asUTCDate(user, rng.Start),
		"end":     asUTCDate(user, rng.End),
		"filter":  pathFilter,
		"browser": browser,
		"limit":   limit + 1,
		"offset":  offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListBrowser")
}

// ListSystems lists OS statistics for the given time period.
func (h *HitStats) ListSystems(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSystems", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
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
func (h *HitStats) ListSystem(ctx context.Context, system string, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSystem", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
		"filter": pathFilter,
		"system": system,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListSystem")
}

const (
	sizePhones      = "phone"
	sizeLargePhones = "largephone"
	sizeTablets     = "tablet"
	sizeDesktop     = "desktop"
	sizeDesktopHD   = "desktophd"
	sizeUnknown     = "unknown"
)

// ListSizes lists all device sizes.
func (h *HitStats) ListSizes(ctx context.Context, rng ztime.Range, pathFilter []int64) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSizes", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
		"filter": pathFilter,
	})
	if err != nil {
		return errors.Wrap(err, "HitStats.ListSize")
	}

	// Group a bit more user-friendly.
	ns := []HitStat{
		{ID: sizePhones, Count: 0},
		{ID: sizeLargePhones, Count: 0},
		{ID: sizeTablets, Count: 0},
		{ID: sizeDesktop, Count: 0},
		{ID: sizeDesktopHD, Count: 0},
		{ID: sizeUnknown, Count: 0},
	}
	for i := range h.Stats {
		x, _ := strconv.ParseInt(h.Stats[i].Name, 10, 16)
		switch {
		case x == 0:
			ns[5].Count += h.Stats[i].Count
		case x <= 384:
			ns[0].Count += h.Stats[i].Count
		case x <= 1024:
			ns[1].Count += h.Stats[i].Count
		case x <= 1440:
			ns[2].Count += h.Stats[i].Count
		case x <= 1920:
			ns[3].Count += h.Stats[i].Count
		default:
			ns[4].Count += h.Stats[i].Count
		}
	}
	h.Stats = ns

	return nil
}

// ListSize lists all sizes for one grouping.
func (h *HitStats) ListSize(ctx context.Context, id string, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	var (
		minSize, maxSize int
		empty            bool
	)
	switch id {
	case sizePhones:
		maxSize = 384
	case sizeLargePhones:
		minSize, maxSize = 384, 1024
	case sizeTablets:
		minSize, maxSize = 1024, 1440
	case sizeDesktop:
		minSize, maxSize = 1440, 1920
	case sizeDesktopHD:
		minSize, maxSize = 1920, 99999
	case sizeUnknown:
		empty = true
	default:
		return errors.Errorf("HitStats.ListSizes: invalid value for name: %#v", id)
	}

	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListSize", map[string]any{
		"site":     MustGetSite(ctx).ID,
		"start":    asUTCDate(user, rng.Start),
		"end":      asUTCDate(user, rng.End),
		"filter":   pathFilter,
		"min_size": minSize,
		"max_size": maxSize,
		"empty":    empty,
		"limit":    limit + 1,
		"offset":   offset,
	})
	if err != nil {
		return errors.Wrap(err, "HitStats.ListSize")
	}
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	for i := range h.Stats { // TODO: see if we can do this in SQL.
		h.Stats[i].Name = strings.ReplaceAll(h.Stats[i].Name, "↔", "↔\ufe0e")
	}
	return nil
}

// ListLocations lists all location statistics for the given time period.
func (h *HitStats) ListLocations(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListLocations", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
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
func (h *HitStats) ListLocation(ctx context.Context, country string, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListLocation", map[string]any{
		"site":    MustGetSite(ctx).ID,
		"start":   asUTCDate(user, rng.Start),
		"end":     asUTCDate(user, rng.End),
		"filter":  pathFilter,
		"country": country,
		"limit":   limit + 1,
		"offset":  offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListLocation")
}

// ListLanguages lists all language statistics for the given time period.
func (h *HitStats) ListLanguages(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListLanguages", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
		"filter": pathFilter,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListLanguages")
}

// ListCampaigns lists all campaigns statistics for the given time period.
func (h *HitStats) ListCampaigns(ctx context.Context, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListCampaigns", map[string]any{
		"site":   MustGetSite(ctx).ID,
		"start":  asUTCDate(user, rng.Start),
		"end":    asUTCDate(user, rng.End),
		"filter": pathFilter,
		"limit":  limit + 1,
		"offset": offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListCampaigns")
}

// ListCampaign lists all statistics for a campaign.
func (h *HitStats) ListCampaign(ctx context.Context, campaign int64, rng ztime.Range, pathFilter []int64, limit, offset int) error {
	user := MustGetUser(ctx)
	err := zdb.Select(ctx, &h.Stats, "load:hit_stats.ListCampaign", map[string]any{
		"site":     MustGetSite(ctx).ID,
		"start":    asUTCDate(user, rng.Start),
		"end":      asUTCDate(user, rng.End),
		"filter":   pathFilter,
		"campaign": campaign,
		"limit":    limit + 1,
		"offset":   offset,
	})
	if len(h.Stats) > limit {
		h.More = true
		h.Stats = h.Stats[:len(h.Stats)-1]
	}
	return errors.Wrap(err, "HitStats.ListCampaign")
}

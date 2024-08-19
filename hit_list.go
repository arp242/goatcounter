// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"sort"
	"strconv"
	"time"

	"zgo.at/errors"
	"zgo.at/tz"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztime"
)

type HitList struct {
	// Number of visitors for the selected date range.
	Count int `db:"count" json:"count"`

	// Path ID
	PathID int64 `db:"path_id" json:"path_id"`

	// Path name (e.g. /hello.html).
	Path string `db:"path" json:"path"`

	// Is this an event?
	Event zbool.Bool `db:"event" json:"event"`

	// Page title.
	Title string `db:"title" json:"title"`

	// Highest visitors per hour or day (depending on daily being set).
	Max int `json:"max"`

	// Statistics by day and hour.
	Stats []HitListStat `json:"stats"`

	// What kind of referral this is; only set when retrieving referrals {enum: h g c o}.
	//
	//  h   HTTP Referal header.
	//  g   Generated; for example are Google domains (google.com, google.nl,
	//      google.co.nz, etc.) are grouped as the generated referral "Google".
	//  c   Campaign (via query parameter)
	//  o   Other
	RefScheme *string `db:"ref_scheme" json:"ref_scheme,omitempty"`
}

type HitListStat struct {
	Day    string `json:"day"`    // Day these statistics are for {date}.
	Hourly []int  `json:"hourly"` // Visitors per hour.
	Daily  int    `json:"daily"`  // Total visitors for this day.
}

// PathCount gets the visit count for one path.
func (h *HitList) PathCount(ctx context.Context, path string, rng ztime.Range) error {
	err := zdb.Get(ctx, h, "load:hit_list.PathCount", map[string]any{
		"site":  MustGetSite(ctx).ID,
		"path":  path,
		"start": rng.Start,
		"end":   rng.End,
	})
	return errors.Wrap(err, "HitList.PathCount")
}

// SiteTotal gets the total counts for all paths. This always uses UTC.
func (h *HitList) SiteTotalUTC(ctx context.Context, rng ztime.Range) error {
	err := zdb.Get(ctx, h, `/* *HitList.SiteTotalUTC */
			select
				coalesce(sum(total), 0) as count
			from hit_counts
			where site_id = :site
			{{:start and hour >= :start}}
			{{:end   and hour <= :end}}
		`, map[string]any{
		"site":  MustGetSite(ctx).ID,
		"start": rng.Start,
		"end":   rng.End,
	})
	return errors.Wrap(err, "HitList.SiteTotalUTC")
}

type HitLists []HitList

// ListPathsLike lists all paths matching the like pattern.
func (h *HitLists) ListPathsLike(ctx context.Context, search string, matchTitle, matchCase bool) error {
	err := zdb.Select(ctx, h, "load:hit_list.ListPathsLike", map[string]any{
		"site":        MustGetSite(ctx).ID,
		"search":      search,
		"match_title": matchTitle,
		"match_case":  matchCase,
	})
	return errors.Wrap(err, "Hits.ListPathsLike")
}

var allDays = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// List the top paths for this site in the given time period.
func (h *HitLists) List(
	ctx context.Context, rng ztime.Range, pathFilter, exclude []int64, limit int, daily bool,
) (int, bool, error) {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)

	// List the pages for this time period; this gets the path_id, path, title.
	var more bool
	{
		err := zdb.Select(ctx, h, "load:hit_list.List-counts", map[string]any{
			"site":    site.ID,
			"start":   rng.Start,
			"end":     rng.End,
			"filter":  pathFilter,
			"limit":   limit + 1,
			"exclude": exclude,
		})
		if err != nil {
			return 0, false, errors.Wrap(err, "HitLists.List hit_counts")
		}

		// Check if there are more entries.
		if len(*h) > limit {
			hh := *h
			hh = hh[:len(hh)-1]
			*h = hh
			more = true
		}
	}

	if len(*h) == 0 { // No data yet.
		return 0, false, nil
	}

	// Get stats for every page.
	hh := *h
	var st []struct {
		PathID int64     `db:"path_id"`
		Day    time.Time `db:"day"`
		Stats  []byte    `db:"stats"`
	}
	{
		paths := make([]int64, len(hh))
		for i := range hh {
			paths[i] = hh[i].PathID
		}

		err := zdb.Select(ctx, &st, "load:hit_list.List-stats", map[string]any{
			"site":  site.ID,
			"start": rng.Start.Format("2006-01-02"),
			"end":   rng.End.Format("2006-01-02"),
			"paths": paths,
		})
		if err != nil {
			return 0, false, errors.Wrap(err, "HitLists.List hit_stats")
		}
	}

	// Add the hit_stats.
	{
		for i := range hh {
			for _, s := range st {
				if s.PathID == hh[i].PathID {
					var y []int
					zjson.MustUnmarshal(s.Stats, &y)
					hh[i].Stats = append(hh[i].Stats, HitListStat{
						Day:    s.Day.Format("2006-01-02"),
						Hourly: y,
					})
				}
			}
		}
	}

	fillBlankDays(hh, rng)
	applyOffset(hh, user.Settings.Timezone)

	// Add total and max.
	var totalDisplay int
	addTotals(hh, daily, &totalDisplay)

	return totalDisplay, more, nil
}

// PathTotals is a special path to indicate this is the "total" overview.
//
// Trailing whitespace is trimmed on paths, so this should never conflict.
const PathTotals = "TOTAL "

// Totals gets the data for the "Totals" chart/widget.
func (h *HitList) Totals(ctx context.Context, rng ztime.Range, pathFilter []int64, daily, noEvents bool) (int, error) {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)

	var tc []struct {
		Hour  time.Time `db:"hour"`
		Total int       `db:"total"`
	}
	err := zdb.Select(ctx, &tc, "load:hit_list.Totals", map[string]any{
		"site":      site.ID,
		"start":     rng.Start,
		"end":       rng.End,
		"filter":    pathFilter,
		"no_events": noEvents,
	})
	if err != nil {
		return 0, errors.Wrap(err, "HitList.Totals")
	}

	totalst := HitList{
		Path:  PathTotals,
		Title: "",
	}
	stats := make(map[string]HitListStat)
	for _, t := range tc {
		d := t.Hour.Format("2006-01-02")
		hour, _ := strconv.ParseInt(t.Hour.Format("15"), 10, 32)
		s, ok := stats[d]
		if !ok {
			s = HitListStat{
				Day:    d,
				Hourly: make([]int, 24),
			}
		}

		s.Hourly[hour] += t.Total
		totalst.Count += t.Total

		stats[d] = s
	}

	max := 0
	for _, v := range stats {
		totalst.Stats = append(totalst.Stats, v)
		if !daily {
			for _, x := range v.Hourly {
				if x > max {
					max = x
				}
			}
		}
	}

	sort.Slice(totalst.Stats, func(i, j int) bool {
		return totalst.Stats[i].Day < totalst.Stats[j].Day
	})

	hh := []HitList{totalst}
	fillBlankDays(hh, rng)
	applyOffset(hh, user.Settings.Timezone)

	if daily {
		for i := range hh[0].Stats {
			for _, n := range hh[0].Stats[i].Hourly {
				hh[0].Stats[i].Daily += n
			}
			if daily && hh[0].Stats[i].Daily > max {
				max = hh[0].Stats[i].Daily
			}
		}
	}

	if max < 10 {
		max = 10
	}

	*h = hh[0]
	return max, nil
}

// The database stores everything in UTC, so we need to apply
// the offset for HitLists.List()
//
// Let's say we have two days with an offset of UTC+2, this means we
// need to transform this:
//
//	2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//	2019-12-06 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//	2019-12-07 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// To:
//
//	2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//	2019-12-06 → [1,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//	2019-12-07 → [1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// And skip the first 2 hours of the first day.
//
// Or, for UTC-2:
//
//	2019-12-04 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//	2019-12-05 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//	2019-12-06 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//
// And skip the last 2 hours of the last day.
//
// Offsets that are not whole hours (e.g. 6:30) are treated like 7:00. I don't
// know how to do that otherwise.
func applyOffset(hh HitLists, tz *tz.Zone) {
	if len(hh) == 0 {
		return
	}

	offset := tz.Offset()
	if offset%60 != 0 {
		offset += 30
	}
	offset /= 60

	switch {
	case offset > 0:
		for i := range hh {
			stats := hh[i].Stats

			popped := make([]int, offset)
			for i := range stats {
				stats[i].Hourly = append(popped, stats[i].Hourly...)
				o := len(stats[i].Hourly) - offset
				popped = stats[i].Hourly[o:]
				stats[i].Hourly = stats[i].Hourly[:o]
			}
			if len(hh[i].Stats) > 1 {
				hh[i].Stats = stats[1:] // Overselect a day to get the stats for it, remove it.
			}
		}

	case offset < 0:
		offset = -offset

		for i := range hh {
			stats := hh[i].Stats

			popped := make([]int, offset)
			for i := len(stats) - 1; i >= 0; i-- {
				stats[i].Hourly = append(stats[i].Hourly, popped...)
				popped = stats[i].Hourly[:offset]
				stats[i].Hourly = stats[i].Hourly[offset:]
			}
			hh[i].Stats = stats[:len(stats)-1] // Overselect a day to get the stats for it, remove it.
		}
	}
}

func fillBlankDays(hh HitLists, rng ztime.Range) {
	// Should Never Happen™ but if it does the below loop will never break, so
	// be safe.
	if rng.Start.After(rng.End) {
		return
	}

	endFmt := rng.End.Format("2006-01-02")
	for i := range hh {
		var (
			day     = rng.Start.Add(-24 * time.Hour)
			newStat []HitListStat
			j       int
		)

		for {
			day = day.Add(24 * time.Hour)
			dayFmt := day.Format("2006-01-02")

			if len(hh[i].Stats)-1 >= j && dayFmt == hh[i].Stats[j].Day {
				newStat = append(newStat, hh[i].Stats[j])
				j++
			} else {
				newStat = append(newStat, HitListStat{Day: dayFmt, Hourly: allDays})
			}
			if dayFmt == endFmt {
				break
			}
		}

		hh[i].Stats = newStat
	}
}

func addTotals(hh HitLists, daily bool, totalDisplay *int) {
	for i := range hh {
		for j := range hh[i].Stats {
			for k := range hh[i].Stats[j].Hourly {
				hh[i].Stats[j].Daily += hh[i].Stats[j].Hourly[k]
				if !daily && hh[i].Stats[j].Hourly[k] > hh[i].Max {
					hh[i].Max = hh[i].Stats[j].Hourly[k]
				}
			}

			hh[i].Count += hh[i].Stats[j].Daily
			if daily && hh[i].Stats[j].Daily > hh[i].Max {
				hh[i].Max = hh[i].Stats[j].Daily
			}
		}

		*totalDisplay += hh[i].Count
	}

	// We sort in SQL, but this is not always 100% correct after applying
	// the TZ offset, so order here as well.
	//
	// TODO: this is still not 100% correct, as the "first 10" after
	// applying the TZ offset may be different than the first 10 being
	// fetched in the SQL query. There is no easy fix for that in the
	// current design. I considered storing everything in the DB as the
	// configured TZ, but that would make changing the TZ expensive, I'm not
	// 100% sure yet what a good solution here is. For now, this is "good
	// enough".
	sort.Slice(hh, func(i, j int) bool { return hh[i].Count > hh[j].Count })
}

type TotalCount struct {
	Total       int `db:"total" json:"total"`               // Total number of visitors (including events).
	TotalEvents int `db:"total_events" json:"total_events"` // Total number of visitors for events.
	// Total number of visitors in UTC. The browser, system, etc, stats are
	// always in UTC.
	TotalUTC int `db:"total_utc" json:"total_utc"`
}

// GetTotalCount gets the total number of pageviews for the selected timeview in
// the timezone the user configured.
//
// This also gets the total number of pageviews for the selected time period in
// UTC. This is needed since the _stats tables are per day, rather than
// per-hour, so we need to use the correct totals to make sure the percentage
// calculations are accurate.
func GetTotalCount(ctx context.Context, rng ztime.Range, pathFilter []int64, noEvents bool) (TotalCount, error) {
	site := MustGetSite(ctx)
	user := MustGetUser(ctx)

	var t TotalCount
	err := zdb.Get(ctx, &t, "load:hit_list.GetTotalCount", map[string]any{
		"site":      site.ID,
		"start":     rng.Start,
		"end":       rng.End,
		"start_utc": rng.Start.In(user.Settings.Timezone.Location),
		"end_utc":   rng.End.In(user.Settings.Timezone.Location),
		"filter":    pathFilter,
		"no_events": noEvents,
		"tz":        user.Settings.Timezone.Offset(),
	})
	return t, errors.Wrap(err, "GetTotalCount")
}

// Diff gets the difference in percentage of all paths in this HitList.
//
// e.g. if called with start=2020-01-20; end=2020-01-2020-01-27, then it will
// compare this to start=2020-01-12; end=2020-01-19
//
// The return value is in the same order as paths.
func (h HitLists) Diff(ctx context.Context, rng, prev ztime.Range) ([]float64, error) {
	if len(h) == 0 {
		return nil, nil
	}

	d := -rng.End.Sub(rng.Start)
	prev = ztime.NewRange(rng.Start.Add(d)).To(rng.End.Add(d))

	paths := make([]int64, 0, len(h))
	for _, hh := range h {
		paths = append(paths, hh.PathID)
	}

	var diffs []float64
	err := zdb.Select(ctx, &diffs, "load:hit_list.DiffTotal", map[string]any{
		"site":      MustGetSite(ctx).ID,
		"start":     rng.Start,
		"end":       rng.End,
		"prevstart": prev.Start,
		"prevend":   prev.End,
		"paths":     paths,
	})
	return diffs, errors.Wrap(err, "HitList.DiffTotal")
}

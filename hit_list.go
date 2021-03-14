// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"sort"
	"strconv"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
)

type HitList struct {
	Count       int        `db:"count"`
	CountUnique int        `db:"count_unique"`
	PathID      int64      `db:"path_id"`
	Path        string     `db:"path"`
	Event       zbool.Bool `db:"event"`
	Title       string     `db:"title"`
	RefScheme   *string    `db:"ref_scheme"`
	Max         int
	Stats       []HitListStat
}

type HitListStat struct {
	Day          string
	Hourly       []int
	HourlyUnique []int
	Daily        int
	DailyUnique  int
}

type HitLists []HitList

// ListPathsLike lists all paths matching the like pattern.
func (h *HitLists) ListPathsLike(ctx context.Context, search string, matchTitle bool) error {
	err := zdb.Select(ctx, h, "load:hit_list.ListPathsLike", zdb.P{
		"site":        MustGetSite(ctx).ID,
		"search":      search,
		"match_title": matchTitle,
	})
	return errors.Wrap(err, "Hits.ListPathsLike")
}

// PathCountUnique gets the total_unique for one path.
func (h *HitLists) PathCountUnique(ctx context.Context, path string) error {
	err := zdb.Select(ctx, h, "load:hit_list.PathCountUnique", zdb.P{
		"site": MustGetSite(ctx).ID,
		"path": path,
	})
	return errors.Wrap(err, "HitLists.PathCountUnique")
}

// SiteTotalUnique gets the total_unique for all paths.
func (h *HitLists) SiteTotalUnique(ctx context.Context) error {
	err := zdb.Select(ctx, h, `/* *HitLists.SiteTotalUnique */
		select sum(total_unique) as count_unique from hit_counts
		where site_id=$1`, MustGetSite(ctx).ID)
	return errors.Wrap(err, "HitLists.SiteTotalUnique")
}

var allDays = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// List the top paths for this site in the given time period.
func (h *HitLists) List(
	ctx context.Context, start, end time.Time, pathFilter, exclude []int64, daily bool,
) (int, int, bool, error) {
	site := MustGetSite(ctx)

	// List the pages for this page; this gets the path_id, path, title.
	var more bool
	{
		limit := int(zint.NonZero(int64(site.Settings.LimitPages()), 10))
		err := zdb.Select(ctx, h, `/* HitLists.List */
			with x as (
				select path_id from hit_counts
				where
					hit_counts.site_id = :site and
					{{:exclude path_id not in (:exclude) and}}
					{{:filter path_id in (:filter) and}}
					hour>=:start and hour<=:end
				group by path_id
				order by sum(total_unique) desc, path_id desc
				limit :limit
			)
			select path_id, paths.path, paths.title, paths.event from x
			join paths using (path_id)`,
			zdb.P{
				"site":    site.ID,
				"start":   start,
				"end":     end,
				"filter":  pathFilter,
				"limit":   limit + 1,
				"exclude": exclude,
			})
		if err != nil {
			return 0, 0, false, errors.Wrap(err, "HitLists.List hit_counts")
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
		return 0, 0, false, nil
	}

	// Get stats for every page.
	hh := *h
	var st []struct {
		PathID      int64     `db:"path_id"`
		Day         time.Time `db:"day"`
		Stats       []byte    `db:"stats"`
		StatsUnique []byte    `db:"stats_unique"`
	}
	{
		paths := make([]int64, len(hh))
		for i := range hh {
			paths[i] = hh[i].PathID
		}

		err := zdb.Select(ctx, &st, `/* HitLists.List */
			select path_id, day, stats, stats_unique
			from hit_stats
			where
				hit_stats.site_id = :site and
				path_id in (:paths) and
				day >= :start and day <= :end
			order by day asc`,
			zdb.P{
				"site":  site.ID,
				"start": start.Format("2006-01-02"),
				"end":   end.Format("2006-01-02"),
				"paths": paths,
			})
		if err != nil {
			return 0, 0, false, errors.Wrap(err, "HitLists.List hit_stats")
		}
	}

	// Add the hit_stats.
	{
		for i := range hh {
			for _, s := range st {
				if s.PathID == hh[i].PathID {
					var x, y []int
					zjson.MustUnmarshal(s.Stats, &x)
					zjson.MustUnmarshal(s.StatsUnique, &y)
					hh[i].Stats = append(hh[i].Stats, HitListStat{
						Day:          s.Day.Format("2006-01-02"),
						Hourly:       x,
						HourlyUnique: y,
					})
				}
			}
		}
	}

	// Fill in blank days.
	fillBlankDays(hh, start, end)

	// Apply TZ offset.
	applyOffset(hh, *site)

	// Add total and max.
	var totalDisplay, totalUniqueDisplay int
	addTotals(hh, daily, &totalDisplay, &totalUniqueDisplay)

	return totalDisplay, totalUniqueDisplay, more, nil
}

// PathTotals is a special path to indicate this is the "total" overview.
//
// Trailing whitespace is trimmed on paths, so this should never conflict.
const PathTotals = "TOTAL "

// Totals gets the data for the "Totals" chart/widget.
func (h *HitList) Totals(ctx context.Context, start, end time.Time, pathFilter []int64, daily bool) (int, error) {
	site := MustGetSite(ctx)

	var tc []struct {
		Hour        time.Time `db:"hour"`
		Total       int       `db:"total"`
		TotalUnique int       `db:"total_unique"`
	}
	err := zdb.Select(ctx, &tc, "load:hit_list.Totals", zdb.P{
		"site":      site.ID,
		"start":     start,
		"end":       end,
		"filter":    pathFilter,
		"no_events": site.Settings.TotalsNoEvents(),
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
				Day:          d,
				Hourly:       make([]int, 24),
				HourlyUnique: make([]int, 24),
			}
		}

		s.Hourly[hour] += t.Total
		s.HourlyUnique[hour] += t.TotalUnique
		totalst.Count += t.Total
		totalst.CountUnique += t.TotalUnique

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
	fillBlankDays(hh, start, end)
	applyOffset(hh, *site)

	if daily {
		for i := range hh[0].Stats {
			for _, n := range hh[0].Stats[i].Hourly {
				hh[0].Stats[i].Daily += n
			}
			for _, n := range hh[0].Stats[i].HourlyUnique {
				hh[0].Stats[i].DailyUnique += n
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
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//    2019-12-06 → [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
//    2019-12-07 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// To:
//
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//    2019-12-06 → [1,0,0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0]
//    2019-12-07 → [1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//
// And skip the first 2 hours of the first day.
//
// Or, for UTC-2:
//
//    2019-12-04 → [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
//    2019-12-05 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//    2019-12-06 → [0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0,0,0]
//
// And skip the last 2 hours of the last day.
//
// Offsets that are not whole hours (e.g. 6:30) are treated like 7:00. I don't
// know how to do that otherwise.
func applyOffset(hh HitLists, site Site) {
	if len(hh) == 0 {
		return
	}

	offset := site.Settings.Timezone.Offset()
	if offset%60 != 0 {
		offset += 30
	}
	offset /= 60

	switch {
	case offset > 0:
		for i := range hh {
			stats := hh[i].Stats

			popped := make([]int, offset)
			poppedUnique := make([]int, offset)
			for i := range stats {
				stats[i].Hourly = append(popped, stats[i].Hourly...)
				o := len(stats[i].Hourly) - offset
				popped = stats[i].Hourly[o:]
				stats[i].Hourly = stats[i].Hourly[:o]

				stats[i].HourlyUnique = append(poppedUnique, stats[i].HourlyUnique...)
				poppedUnique = stats[i].HourlyUnique[o:]
				stats[i].HourlyUnique = stats[i].HourlyUnique[:o]
			}
			hh[i].Stats = stats[1:] // Overselect a day to get the stats for it, remove it.
		}

	case offset < 0:
		offset = -offset

		for i := range hh {
			stats := hh[i].Stats

			popped := make([]int, offset)
			poppedUnique := make([]int, offset)
			for i := len(stats) - 1; i >= 0; i-- {
				stats[i].Hourly = append(stats[i].Hourly, popped...)
				popped = stats[i].Hourly[:offset]
				stats[i].Hourly = stats[i].Hourly[offset:]

				stats[i].HourlyUnique = append(stats[i].HourlyUnique, poppedUnique...)
				poppedUnique = stats[i].HourlyUnique[:offset]
				stats[i].HourlyUnique = stats[i].HourlyUnique[offset:]
			}
			hh[i].Stats = stats[:len(stats)-1] // Overselect a day to get the stats for it, remove it.
		}
	}
}

func fillBlankDays(hh HitLists, start, end time.Time) {
	// Should Never Happen™ but if it does the below loop will never break, so
	// be safe.
	if start.After(end) {
		return
	}

	endFmt := end.Format("2006-01-02")
	for i := range hh {
		var (
			day     = start.Add(-24 * time.Hour)
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
				newStat = append(newStat, HitListStat{Day: dayFmt, Hourly: allDays, HourlyUnique: allDays})
			}
			if dayFmt == endFmt {
				break
			}
		}

		hh[i].Stats = newStat
	}
}

func addTotals(hh HitLists, daily bool, totalDisplay, totalUniqueDisplay *int) {
	for i := range hh {
		for j := range hh[i].Stats {
			for k := range hh[i].Stats[j].Hourly {
				hh[i].Stats[j].Daily += hh[i].Stats[j].Hourly[k]
				hh[i].Stats[j].DailyUnique += hh[i].Stats[j].HourlyUnique[k]
				if !daily && hh[i].Stats[j].Hourly[k] > hh[i].Max {
					hh[i].Max = hh[i].Stats[j].Hourly[k]
				}
			}

			hh[i].Count += hh[i].Stats[j].Daily
			hh[i].CountUnique += hh[i].Stats[j].DailyUnique
			if daily && hh[i].Stats[j].Daily > hh[i].Max {
				hh[i].Max = hh[i].Stats[j].Daily
			}
		}

		*totalDisplay += hh[i].Count
		*totalUniqueDisplay += hh[i].CountUnique
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
	sort.Slice(hh, func(i, j int) bool { return hh[i].CountUnique > hh[j].CountUnique })
}

type TotalCount struct {
	Total             int `db:"total"`
	TotalUnique       int `db:"total_unique"`
	TotalUniqueUTC    int `db:"total_unique_utc"`
	TotalEvents       int `db:"total_events"`
	TotalEventsUnique int `db:"total_events_unique"`
}

// GetTotalCount gets the total number of pageviews for the selected timeview in
// the timezone the user configured.
//
// This also gets the total number of pageviews for the selected time period in
// UTC. This is needed since the _stats tables are per day, rather than
// per-hour, so we need to use the correct totals to make sure the percentage
// calculations are accurate.
func GetTotalCount(ctx context.Context, start, end time.Time, pathFilter []int64) (TotalCount, error) {
	site := MustGetSite(ctx)

	var t TotalCount
	err := zdb.Get(ctx, &t, "load:hit_list.GetTotalCount", zdb.P{
		"site":      site.ID,
		"start":     start,
		"end":       end,
		"start_utc": start.In(site.Settings.Timezone.Location),
		"end_utc":   end.In(site.Settings.Timezone.Location),
		"filter":    pathFilter,
		"no_events": site.Settings.TotalsNoEvents(),
		"tz":        site.Settings.Timezone.Offset(),
	})
	return t, errors.Wrap(err, "GetTotalCount")
}

// GetMax gets the path with the higest number of pageviews per hour or day for
// this date range.
func GetMax(ctx context.Context, start, end time.Time, pathFilter []int64, daily bool) (int, error) {
	site := MustGetSite(ctx)
	var (
		query  string
		params zdb.P
	)
	if daily {
		query = "load:hit_list.GetMax-daily"
		params = zdb.P{
			"site":   site.ID,
			"start":  start,
			"end":    end,
			"tz":     site.Settings.Timezone.OffsetRFC3339(),
			"filter": pathFilter,
			"pgsql":  zdb.Driver(ctx) == zdb.DriverPostgreSQL,
			"sqlite": zdb.Driver(ctx) == zdb.DriverSQLite,
		}
	} else {
		query = "load:hit_list.GetMax-hourly"
		params = zdb.P{
			"site":   site.ID,
			"start":  start,
			"end":    end,
			"filter": pathFilter,
		}
	}

	var max int
	err := zdb.Get(ctx, &max, query, params)
	if err != nil && !zdb.ErrNoRows(err) {
		return 0, errors.Wrap(err, "getMax")
	}

	if max < 10 {
		max = 10
	}
	return max, nil
}

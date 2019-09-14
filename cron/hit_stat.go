// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"context"
	"time"

	"github.com/jinzhu/now"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/teamwork/utils/jsonutil"
	"github.com/teamwork/utils/sliceutil"
	"zgo.at/zhttp/ctxkey"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bulk"
	"zgo.at/goatcounter/cfg"
)

type stat struct {
	Path      string    `db:"path"`
	Count     int       `db:"count"`
	CreatedAt time.Time `db:"created_at"`
}

func updateHitStats(ctx context.Context, site goatcounter.Site) error {
	ctx = context.WithValue(ctx, ctxkey.Site, &site)
	db := goatcounter.MustGetDB(ctx)

	// Select everything since last update.
	var last string
	if site.LastStat == nil {
		last = "1970-01-01"
	} else {
		last = site.LastStat.Format("2006-01-02")
	}

	var query string
	if cfg.PgSQL {
		query = `
			select
				path,
				count(path) as count,
				cast(substr(cast(created_at as varchar), 0, 14) || ':00:00' as timestamp) as created_at
			from hits
			where
				site=$1 and
				created_at>=$2
			group by path, substr(cast(created_at as varchar), 0, 14)
			order by path, substr(cast(created_at as varchar), 0, 14)`
	} else {
		query = `
			select
				path,
				count(path) as count,
				created_at
			from hits
			where
				site=$1 and
				created_at>=$2
			group by path, strftime('%Y-%m-%d %H', created_at)
			order by path, strftime('%Y-%m-%d %H', created_at)`
	}

	var stats []stat
	err := db.SelectContext(ctx, &stats, query, site.ID, last)
	if err != nil {
		return errors.Wrap(err, "fetch data")
	}

	existing, err := (&goatcounter.HitStats{}).ListPaths(ctx)
	if err != nil {
		return err
	}

	hourly := fillHitBlanks(stats, existing, site.CreatedAt)

	// No data received.
	if len(hourly) == 0 {
		return nil
	}

	// List all paths we already have so we can update them, rather than
	// inserting new.
	var have []string
	err = db.SelectContext(ctx, &have,
		`select path from hit_stats where site=$1`,
		site.ID)
	if err != nil {
		return errors.Wrap(err, "have")
	}

	ins := bulk.NewInsert(ctx, goatcounter.MustGetDB(ctx).(*sqlx.DB),
		"hit_stats", []string{"site", "day", "path", "stats"})
	for path, dayst := range hourly {
		exists := false
		for _, h := range have {
			if h == path {
				exists = true
				break
			}
		}

		var del []string
		for day, st := range dayst {
			ins.Values(site.ID, day, path, jsonutil.MustMarshal(st))
			if exists {
				del = append(del, day)
			}
		}

		// Delete existing.
		if len(del) > 0 {
			query, args, err := sqlx.In(`delete from hit_stats where
				site=? and lower(path)=lower(?) and day in (?)`, site.ID, path, del)
			if err != nil {
				return errors.Wrap(err, "delete 1")
			}
			_, err = db.ExecContext(ctx, db.Rebind(query), args...)
			if err != nil {
				return errors.Wrap(err, "delete 2")
			}
		}
	}
	return ins.Finish()
}

func fillHitBlanks(stats []stat, existing []string, siteCreated time.Time) map[string]map[string][][]int {
	// Convert data to easier structure:
	// {
	//   "jquery.html": map[string][][]int{
	//     "2019-06-22": []{
	// 	     []int{4, 50},
	// 	     []int{6, 4},
	// 	   },
	// 	   "2019-06-23": []{ .. }.
	// 	 },
	// 	 "other.html": { .. },
	// }
	hourly := map[string]map[string][][]int{}
	first := now.BeginningOfDay()
	for _, s := range stats {
		_, ok := hourly[s.Path]
		if !ok {
			hourly[s.Path] = map[string][][]int{}
		}

		if s.CreatedAt.Before(first) {
			first = now.New(s.CreatedAt).BeginningOfDay()
		}

		day := s.CreatedAt.Format("2006-01-02")
		hourly[s.Path][day] = append(hourly[s.Path][day],
			[]int{s.CreatedAt.Hour(), s.Count})
	}

	// Fill in blank days.
	n := now.BeginningOfDay()
	alldays := []string{first.Format("2006-01-02")}
	for first.Before(n) {
		first = first.Add(24 * time.Hour)
		alldays = append(alldays, first.Format("2006-01-02"))
	}
	allhours := make([][]int, 24)
	for i := 0; i <= 23; i++ {
		allhours[i] = []int{i, 0}
	}
	for path, days := range hourly {
		for _, day := range alldays {
			_, ok := days[day]
			if !ok {
				hourly[path][day] = allhours
			}
		}

		// Backlog new paths since site start.
		// TODO: would be better to modify display logic, instead of storing
		// heaps of data we don't use.
		if !sliceutil.InStringSlice(existing, path) {
			ndays := int(time.Now().Sub(siteCreated) / time.Hour / 24)
			daysSinceCreated := make([]string, ndays)
			for i := 0; i < ndays; i++ {
				daysSinceCreated[i] = siteCreated.Add(24 * time.Duration(i) * time.Hour).Format("2006-01-02")
			}

			for _, day := range daysSinceCreated {
				if _, ok := hourly[path][day]; !ok {
					hourly[path][day] = allhours
				}
			}
		}
	}

	// Fill in blank hours.
	for path, days := range hourly {
		for dayk, day := range days {
			if len(day) == 24 {
				continue
			}

			newday := make([][]int, 24)
		outer:
			for i, hour := range allhours {
				for _, h := range day {
					if h[0] == hour[0] {
						newday[i] = h
						continue outer
					}
				}
				newday[i] = hour
			}

			hourly[path][dayk] = newday
		}
	}

	return hourly
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"context"
	"time"

	"github.com/jinzhu/now"
	"zgo.at/goatcounter"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/sliceutil"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

type stat struct {
	Path      string    `db:"path"`
	Count     int       `db:"count"`
	CreatedAt time.Time `db:"created_at"`
}

// Hit stats are stored per day/path, the value is a 2-tuple: it lists the
// counts for every hour. The first is the hour, second the number of hits in
// that hour:
//
//   site       | 1
//   day        | 2019-12-05
//   path       | /jquery.html
//   stats      | [[0,0],[1,2],[2,2],[3,0],[4,0],[5,0],[6,1],[7,2],[8,3],
//                 [9,0],[10,2],[11,2],[12,2],[13,5],[14,4],[15,3],[16,0],
//                 [17,1],[18,2],[19,0],[20,0],[21,1],[22,4],[23,2]]
func updateHitStats(ctx context.Context, phits map[string][]goatcounter.Hit) error {
	txctx, tx, err := zdb.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_ = txctx

	// Group by day + path.
	type gt struct {
		// TODO: count should be [][]int
		count int
		day   string
		path  string
	}
	grouped := map[string]gt{}
	for _, hits := range phits {
		for _, h := range hits {
			day := h.CreatedAt.Format("2006-01-02")
			k := day + h.Path
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.path = h.Path

				// TODO
				// Append existing and delete from DB; this will be faster than
				// running an update for every row.
				// var c int
				// err := tx.GetContext(txctx, &c, `select count from location_stats where site=$1 and day=$2 and location=$3`,
				// 	h.Site, day, v.location)
				// if err != sql.ErrNoRows {
				// 	if err != nil {
				// 		return errors.Wrap(err, "existing")
				// 	}
				// 	_, err = tx.ExecContext(txctx, `delete from location_stats where site=$1 and day=$2 and location=$3`,
				// 		h.Site, day, v.location)
				// 	if err != nil {
				// 		return errors.Wrap(err, "delete")
				// 	}
				// }

				//v.count = c
			}

			v.count += 1
			grouped[k] = v
		}
	}

	// TODO
	// hourly := fillHitBlanks(stats, existing, site.CreatedAt)

	// // No data received.
	// if len(hourly) == 0 {
	// 	return nil
	// }

	siteID := goatcounter.MustGetSite(ctx).ID
	ins := bulk.NewInsert(ctx, zdb.MustGet(ctx),
		"hit_stats", []string{"site", "day", "path", "stats"})
	for _, v := range grouped {
		ins.Values(siteID, v.day, v.path, jsonutil.MustMarshal(v.count))
	}
	err = ins.Finish()
	if err != nil {
		return err
	}

	return tx.Commit()
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
			ndays := int(time.Now().UTC().Sub(siteCreated) / time.Hour / 24)
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

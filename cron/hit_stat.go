// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"
	"strconv"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/errors"
	"zgo.at/utils/jsonutil"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Hit stats are stored per day/path, the value is a 2-tuple: it lists the
// counts for every hour. The first is the hour, second the number of hits in
// that hour:
//
//   site       | 1
//   day        | 2019-12-05
//   path       | /jquery.html
//   title      | Why I'm still using jQuery in 2019
//   stats      | [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
func updateHitStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + path + event.
		type gt struct {
			count       []int
			countUnique []int
			total       int
			totalUnique int
			day         string
			hour        string
			event       zdb.Bool
			path        string
			title       string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			dayHour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := fmt.Sprintf("%s%s%t", day, h.Path, h.Event)
			v := grouped[k]
			if len(v.count) == 0 {
				v.day = day
				v.hour = dayHour
				v.path = h.Path
				v.event = h.Event
				var err error
				v.count, v.countUnique, v.title, err = existingHitStats(ctx, tx,
					h.Site, day, v.path, v.event)
				if err != nil {
					return err
				}
				for _, i := range v.count {
					v.total += i
				}
				for _, i := range v.countUnique {
					v.totalUnique += i
				}
			}

			if h.Title != "" {
				v.title = h.Title
			}

			hour, _ := strconv.ParseInt(h.CreatedAt.Format("15"), 10, 8)
			v.count[hour] += 1
			v.total += 1
			if h.FirstVisit {
				v.totalUnique += 1
				v.countUnique[hour] += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "hit_stats", []string{"site", "day", "path",
			"event", "title", "stats", "stats_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.path, v.event, v.title,
				jsonutil.MustMarshal(v.count),
				jsonutil.MustMarshal(v.countUnique))
		}
		return errors.Wrap(ins.Finish(), "updateHitStats hit_stats")
	})
}

func existingHitStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, path string, event zdb.Bool,
) ([]int, []int, string, error) {

	var ex []struct {
		Stats       []byte   `db:"stats"`
		StatsUnique []byte   `db:"stats_unique"`
		Title       string   `db:"title"`
		Event       zdb.Bool `db:"event"`
	}
	err := tx.SelectContext(txctx, &ex, `/* existingHitStats */
		select stats, stats_unique, title, event from hit_stats
		where site=$1 and day=$2 and path=$3 and event=$4 limit 1`,
		siteID, day, path, event)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "existingHitStats")
	}
	if len(ex) == 0 {
		return make([]int, 24), make([]int, 24), "", nil
	}

	_, err = tx.ExecContext(txctx, `delete from hit_stats where
		site=$1 and day=$2 and path=$3 and event=$4`,
		siteID, day, path, event)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "delete")
	}

	var r, ru []int
	if ex[0].Stats != nil {
		jsonutil.MustUnmarshal(ex[0].Stats, &r)
		jsonutil.MustUnmarshal(ex[0].StatsUnique, &ru)
	}

	return r, ru, ex[0].Title, nil
}

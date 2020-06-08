// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zstd/zjson"
)

// Hit stats are stored per day/path, the value is a 2-tuple: it lists the
// counts for every hour. The first is the hour, second the number of hits in
// that hour:
//
//   site       | 1
//   day        | 2019-12-05
//   path_id    | 42
//   stats      | [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
func updateHitStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + path.
		type gt struct {
			count       []int
			countUnique []int
			day         string
			hour        string
			pathID      int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			dayHour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := day + strconv.FormatInt(h.PathID, 10)
			v := grouped[k]
			if len(v.count) == 0 {
				v.day = day
				v.hour = dayHour
				v.pathID = h.PathID
				var err error
				v.count, v.countUnique, err = existingHitStats(ctx, tx,
					h.Site, day, v.pathID)
				if err != nil {
					return err
				}
			}

			hour, _ := strconv.ParseInt(h.CreatedAt.Format("15"), 10, 8)
			v.count[hour] += 1
			if h.FirstVisit {
				v.countUnique[hour] += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "hit_stats", []string{"site", "day", "path_id",
			"stats", "stats_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.pathID,
				zjson.MustMarshal(v.count),
				zjson.MustMarshal(v.countUnique))
		}
		return errors.Wrap(ins.Finish(), "updateHitStats hit_stats")
	})
}

func existingHitStats(
	txctx context.Context, tx zdb.DB,
	siteID int64, day string, pathID int64,
) ([]int, []int, error) {

	var ex []struct {
		Stats       []byte `db:"stats"`
		StatsUnique []byte `db:"stats_unique"`
	}
	err := tx.SelectContext(txctx, &ex, `/* existingHitStats */
		select stats, stats_unique from hit_stats
		where site=$1 and day=$2 and path_id=$3 limit 1`,
		siteID, day, pathID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "existingHitStats")
	}
	if len(ex) == 0 {
		return make([]int, 24), make([]int, 24), nil
	}

	_, err = tx.ExecContext(txctx, `delete from hit_stats where
		site=$1 and day=$2 and path_id=$3`,
		siteID, day, pathID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "delete")
	}

	var r, ru []int
	if ex[0].Stats != nil {
		zjson.MustUnmarshal(ex[0].Stats, &r)
		zjson.MustUnmarshal(ex[0].StatsUnique, &ru)
	}

	return r, ru, nil
}

// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
)

func updateHitStats(ctx context.Context, hits []goatcounter.Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count  []int
			total  int
			day    string
			hour   string
			pathID int64
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
				v.count = make([]int, 24)

				if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
					var err error
					v.count, err = existingHitStats(ctx, h.Site, day, v.pathID)
					if err != nil {
						return err
					}
				}
			}

			hour, _ := strconv.ParseInt(h.CreatedAt.Format("15"), 10, 8)
			if h.FirstVisit {
				v.count[hour] += 1
				v.total += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := zdb.NewBulkInsert(ctx, "hit_stats", []string{"site_id", "day", "path_id", "stats"})
		if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
			ins.OnConflict(`on conflict on constraint "hit_stats#site_id#path_id#day" do update set
				stats = (
					with x as (
						select
							unnest(string_to_array(trim(hit_stats.stats, '[]'), ',')::int[]) as orig,
							unnest(string_to_array(trim(excluded.stats,  '[]'), ',')::int[]) as new
					)
					select '[' || array_to_string(array_agg(orig + new), ',') || ']' from x
				) `)
		}
		// } else {
		// TODO: merge the arrays here and get rid of existingHitStats();
		// it's kinda tricky with SQLite :-/
		//
		// ins.OnConflict(`on conflict(site_id, path_id, day) do update set
		// 	stats = excluded.stats
		// `)
		// }

		for _, v := range grouped {
			if v.total > 0 {
				ins.Values(siteID, v.day, v.pathID, zjson.MustMarshal(v.count))
			}
		}
		return errors.Wrap(ins.Finish(), "updateHitStats hit_stats")
	}), "cron.updateHitStats")
}

func existingHitStats(ctx context.Context, siteID int64, day string, pathID int64) ([]int, error) {
	var ex []struct {
		Stats []byte `db:"stats"`
	}
	err := zdb.Select(ctx, &ex, `/* existingHitStats */
		select stats from hit_stats
		where site_id=$1 and day=$2 and path_id=$3 limit 1`,
		siteID, day, pathID)
	if err != nil {
		return nil, errors.Wrap(err, "existingHitStats")
	}
	if len(ex) == 0 {
		return make([]int, 24), nil
	}

	err = zdb.Exec(ctx, `delete from hit_stats where
		site_id=$1 and day=$2 and path_id=$3`,
		siteID, day, pathID)
	if err != nil {
		return nil, errors.Wrap(err, "delete")
	}

	var ru []int
	if ex[0].Stats != nil {
		zjson.MustUnmarshal(ex[0].Stats, &ru)
	}

	return ru, nil
}

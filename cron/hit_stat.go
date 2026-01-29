package cron

import (
	"context"
	"slices"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
	"zgo.at/zstd/zjson"
)

var empty = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func updateHitStats(ctx context.Context, hits []goatcounter.Hit) error {
	err := zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count  []int
			day    string
			hour   string
			pathID goatcounter.PathID
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			dayHour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := day + strconv.Itoa(int(h.PathID))
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
			}
			grouped[k] = v
		}

		ins, err := goatcounter.Tables.HitStats.Bulk(ctx)
		if err != nil {
			return err
		}

		if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
			// TODO: merge the arrays here and get rid of existingHitStats();
			// it's kinda tricky with SQLite :-/
			ins, err = zdb.NewBulkInsert(ctx, "hit_stats", []string{"site_id", "path_id", "day", "stats"})
			if err != nil {
				return err
			}
		}
		siteID := goatcounter.MustGetSite(ctx).ID
		for _, v := range grouped {
			if slices.Equal(v.count, empty) {
				continue
			}
			if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
				ins.Values(siteID, v.pathID, v.day, zjson.MustMarshal(v.count))
			} else {
				ins.Values(siteID, v.pathID, v.day, string(zjson.MustMarshal(v.count)))
			}
		}
		return errors.Wrap(ins.Finish(), "updateHitStats hit_stats")
	})
	return errors.Wrap(err, "cron.updateHitStats")
}

func existingHitStats(ctx context.Context, siteID goatcounter.SiteID, day string, pathID goatcounter.PathID) ([]int, error) {
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

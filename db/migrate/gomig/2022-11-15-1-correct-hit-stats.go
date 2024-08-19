// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package gomig

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/zdb"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zjson"
)

func CorrectHitStats(ctx context.Context) error {
	// Only for SQLite
	if zdb.SQLDialect(ctx) == zdb.DialectPostgreSQL {
		return nil
	}

	err := zdb.TX(goatcounter.NewCache(goatcounter.NewConfig(ctx)), func(ctx context.Context) error {
		err := zdb.Exec(ctx, `delete from hit_stats where day >= '2022-11-08'`)
		if err != nil {
			return err
		}

		var sites goatcounter.Sites
		err = sites.UnscopedList(ctx)
		if err != nil {
			return err
		}

		for _, s := range sites {
			var hits []Hit
			err = zdb.Select(ctx, &hits, `select * from hits where
				created_at >= '2022-11-08 00:00:00' and first_visit=1 and bot in (0, 1)`)
			if err != nil {
				return err
			}

			err = updateHitStats(goatcounter.WithSite(ctx, &s), hits)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err == nil {
		err = zdb.Exec(ctx, `insert into version values ('2022-11-15-1-correct-hit-stats')`)
	}
	return err
}

// below is a copy of cron/hit_stat.go

var empty = []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

func updateHitStats(ctx context.Context, hits []Hit) error {
	return errors.Wrap(zdb.TX(ctx, func(ctx context.Context) error {
		type gt struct {
			count  []int
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
			if slices.Equal(v.count, empty) {
				continue
			}
			ins.Values(siteID, v.day, v.pathID, zjson.MustMarshal(v.count))
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

type Hit struct {
	ID          int64        `db:"hit_id" json:"-"`
	Site        int64        `db:"site_id" json:"-"`
	PathID      int64        `db:"path_id" json:"-"`
	UserAgentID *int64       `db:"user_agent_id" json:"-"`
	CampaignID  *int64       `db:"campaign" json:"-"`
	Session     zint.Uint128 `db:"session" json:"-"`

	Path  string             `db:"-" json:"p,omitempty"`
	Title string             `db:"-" json:"t,omitempty"`
	Ref   string             `db:"ref" json:"r,omitempty"`
	Event zbool.Bool         `db:"-" json:"e,omitempty"`
	Size  goatcounter.Floats `db:"size" json:"s,omitempty"`
	Query string             `db:"-" json:"q,omitempty"`
	Bot   int                `db:"bot" json:"b,omitempty"`

	RefScheme       *string    `db:"ref_scheme" json:"-"`
	UserAgentHeader string     `db:"-" json:"-"`
	Location        string     `db:"location" json:"-"`
	Language        *string    `db:"language" json:"-"`
	FirstVisit      zbool.Bool `db:"first_visit" json:"-"`
	CreatedAt       time.Time  `db:"created_at" json:"-"`

	RefURL *url.URL `db:"-" json:"-"`   // Parsed Ref
	Random string   `db:"-" json:"rnd"` // Browser cache buster, as they don't always listen to Cache-Control

	// Some values we need to pass from the HTTP handler to memstore
	RemoteAddr    string `db:"-" json:"-"`
	UserSessionID string `db:"-" json:"-"`
	BrowserID     int64  `db:"-" json:"-"`
	SystemID      int64  `db:"-" json:"-"`

	// Don't process in memstore; for merging paths.
	//noProcess bool `db:"-" json:"-"`
}

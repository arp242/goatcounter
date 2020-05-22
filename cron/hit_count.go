// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
)

func updateHitCounts(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + path + event.
		type gt struct {
			total       int
			totalUnique int
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

			hour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := fmt.Sprintf("%s%s%t", hour, h.Path, h.Event)
			v := grouped[k]
			if v.total == 0 {
				v.hour = hour
				v.path = h.Path
				v.event = h.Event
				var err error
				v.total, v.totalUnique, err = existingHitCounts(ctx, tx,
					h.Site, hour, v.path, v.event)
				if err != nil {
					return err
				}
			}

			if h.Title != "" {
				v.title = h.Title
			}

			v.total += 1
			if h.FirstVisit {
				v.totalUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		for _, v := range grouped {
			var err error
			if cfg.PgSQL {
				_, err = zdb.MustGet(ctx).ExecContext(ctx, `insert into hit_counts
				(site, path, title, event, hour, total, total_unique) values ($1, $2, $3, $4, $5, $6, $7)
				on conflict on constraint "hit_counts#site#path#hour#event" do
					update set total=$8, total_unique=$9`,
					siteID, v.path, v.title, v.event, v.hour, v.total, v.totalUnique,
					v.total, v.totalUnique)
			} else {
				// SQLite has "on conflict replace" on the unique constraint to
				// do the same.
				_, err = zdb.MustGet(ctx).ExecContext(ctx, `insert into hit_counts
					(site, path, title, event, hour, total, total_unique) values ($1, $2, $3, $4, $5, $6, $7)`,
					siteID, v.path, v.title, v.event, v.hour, v.total, v.totalUnique)
			}
			if err != nil {
				return errors.Wrap(err, "updateHitCounts hit_counts")
			}
		}
		return nil
	})
}

func existingHitCounts(
	txctx context.Context, tx zdb.DB, siteID int64,
	hour, path string, event zdb.Bool,
) (int, int, error) {

	var t, tu int
	row := tx.QueryRowxContext(txctx, `/* existingHitCounts */
		select total, total_unique from hit_counts
		where site=$1 and hour=$2 and path=$3 and event=$4 limit 1`,
		siteID, hour, path, event)
	if err := row.Err(); err != nil {
		if zdb.ErrNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, errors.Wrap(err, "existingHitCounts")
	}

	err := row.Scan(&t, &tu)
	if err != nil {
		if zdb.ErrNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, errors.Wrap(err, "existingHitCounts")
	}

	return t, tu, nil
}

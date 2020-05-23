// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
)

func updateRefCounts(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + path + ref.
		type gt struct {
			total       int
			totalUnique int
			hour        string
			path        string
			ref         string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			hour := h.CreatedAt.Format("2006-01-02 15:00:00")
			k := hour + h.Path + h.Ref
			v := grouped[k]
			if v.total == 0 {
				v.hour = hour
				v.path = h.Path
				var err error
				v.total, v.totalUnique, err = existingRefCounts(ctx, tx,
					h.Site, hour, v.path, v.ref)
				if err != nil {
					return err
				}
			}

			if h.Ref != "" {
				v.ref = h.Ref
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
				_, err = zdb.MustGet(ctx).ExecContext(ctx, `insert into ref_counts
				(site, path, ref, hour, total, total_unique) values ($1, $2, $3, $4, $5, $6)
				on conflict on constraint "ref_counts#site#path#ref#hour" do
					update set total=$7, total_unique=$8`,
					siteID, v.path, v.ref, v.hour, v.total, v.totalUnique,
					v.total, v.totalUnique)
			} else {
				// SQLite has "on conflict replace" on the unique constraint to
				// do the same.
				_, err = zdb.MustGet(ctx).ExecContext(ctx, `insert into ref_counts
					(site, path, ref, hour, total, total_unique) values ($1, $2, $3, $4, $5, $6)`,
					siteID, v.path, v.ref, v.hour, v.total, v.totalUnique)
			}
			if err != nil {
				return errors.Wrap(err, "updateRefCounts ref_counts")
			}
		}
		return nil
	})
}

func existingRefCounts(
	txctx context.Context, tx zdb.DB, siteID int64,
	hour, path, ref string,
) (int, int, error) {

	var t, tu int
	row := tx.QueryRowxContext(txctx, `/* existingRefCounts */
		select total, total_unique from ref_counts
		where site=$1 and hour=$2 and path=$3 and ref=$4 limit 1`,
		siteID, hour, path, ref)
	if err := row.Err(); err != nil {
		if zdb.ErrNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, errors.Wrap(err, "existingRefCounts")
	}

	err := row.Scan(&t, &tu)
	if err != nil {
		if zdb.ErrNoRows(err) {
			return 0, 0, nil
		}
		return 0, 0, errors.Wrap(err, "existingRefCounts")
	}

	return t, tu, nil
}

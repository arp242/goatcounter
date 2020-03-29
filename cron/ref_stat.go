// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Ref stats are stored as a simple day/location with a count.
//  site |    day     | ref      | count
// ------+------------+----------+-------
//     1 | 2019-11-30 | https:// |     1
//     1 | 2019-11-30 | t.co/..  |     2
//     1 | 2019-11-30 | ....     |     4
func updateRefStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + ref.
		type gt struct {
			count int
			day   string
			ref   string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + h.Ref
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.ref = h.Ref
				var err error
				v.count, err = existingRefStats(ctx, tx, h.Site, day, v.ref)
				if err != nil {
					return err
				}
			}

			v.count += 1
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, tx,
			"ref_stats", []string{"site", "day", "ref", "count"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.ref, v.count)
		}
		return ins.Finish()
	})
}

func existingRefStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, ref string,
) (int, error) {

	var c int
	err := tx.GetContext(txctx, &c,
		`select count from ref_stats where site=$1 and day=$2 and ref=$3`,
		siteID, day, ref)
	if err != nil && err != sql.ErrNoRows {
		return 0, errors.Wrap(err, "existing")
	}

	if err != sql.ErrNoRows {
		_, err = tx.ExecContext(txctx,
			`delete from ref_stats where site=$1 and day=$2 and ref=$3`,
			siteID, day, ref)
		if err != nil {
			return 0, errors.Wrap(err, "delete")
		}
	}

	return c, nil
}

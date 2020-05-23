// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"

	"zgo.at/errors"
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
			count       int
			countUnique int
			day         string
			ref         string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := fmt.Sprintf("%s%s", day, h.Ref)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.ref = h.Ref
				var err error
				v.count, v.countUnique, err = existingRefStats(ctx, tx, h.Site,
					day, v.ref)
				if err != nil {
					return err
				}
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "ref_stats", []string{"site", "day", "ref",
			"count", "count_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.ref, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

func existingRefStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, ref string,
) (int, int, error) {

	var c []struct {
		Count       int `db:"count"`
		CountUnique int `db:"count_unique"`
	}
	err := tx.SelectContext(txctx, &c, `/* existingRefStats */
		select count, count_unique from ref_stats
		where site=$1 and day=$2 and ref=$3 limit 1`,
		siteID, day, ref)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from ref_stats where
		site=$1 and day=$2 and ref=$3`,
		siteID, day, ref)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

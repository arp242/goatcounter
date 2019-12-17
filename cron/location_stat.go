// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Location stats are stored as a simple day/location with a count.
//  site |    day     | location | count
// ------+------------+----------+-------
//     1 | 2019-11-30 | ET       |     1
//     1 | 2019-11-30 | GR       |     2
//     1 | 2019-11-30 | MX       |     4
func updateLocationStats(ctx context.Context, phits map[string][]goatcounter.Hit) error {
	txctx, tx, err := zdb.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Group by day + location.
	type gt struct {
		count    int
		day      string
		location string
	}
	grouped := map[string]gt{}
	for _, hits := range phits {
		for _, h := range hits {
			day := h.CreatedAt.Format("2006-01-02")
			k := day + h.Location
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.location = h.Location

				// Append existing and delete from DB; this will be faster than
				// running an update for every row.
				err := tx.GetContext(txctx, &v.count,
					`select count from location_stats where site=$1 and day=$2 and location=$3`,
					h.Site, day, v.location)
				if err != sql.ErrNoRows {
					if err != nil {
						return errors.Wrap(err, "existing")
					}
					_, err = tx.ExecContext(txctx,
						`delete from location_stats where site=$1 and day=$2 and location=$3`,
						h.Site, day, v.location)
					if err != nil {
						return errors.Wrap(err, "delete")
					}
				}
			}

			v.count += 1
			grouped[k] = v
		}
	}

	siteID := goatcounter.MustGetSite(ctx).ID
	ins := bulk.NewInsert(txctx, tx,
		"location_stats", []string{"site", "day", "location", "count"})
	for _, v := range grouped {
		ins.Values(siteID, v.day, v.location, v.count)
	}
	err = ins.Finish()
	if err != nil {
		return err
	}

	return tx.Commit()
}

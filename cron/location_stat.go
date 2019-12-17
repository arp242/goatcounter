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
				v.count, err = existingLocationStats(ctx, tx, h.Site, day, v.location)
				if err != nil {
					return err
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

func existingLocationStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, location string,
) (int, error) {

	var c int
	err := tx.GetContext(txctx, &c,
		`select count from location_stats where site=$1 and day=$2 and location=$3`,
		siteID, day, location)
	if err != nil && err != sql.ErrNoRows {
		return 0, errors.Wrap(err, "existing")
	}

	if err != sql.ErrNoRows {
		_, err = tx.ExecContext(txctx,
			`delete from location_stats where site=$1 and day=$2 and location=$3`,
			siteID, day, location)
		if err != nil {
			return 0, errors.Wrap(err, "delete")
		}
	}

	return c, nil
}

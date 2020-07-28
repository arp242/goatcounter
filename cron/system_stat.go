// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"strconv"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// TODO: add path_id here too?

func updateSystemStats(ctx context.Context, hits []goatcounter.Hit, isReindex bool) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + system.
		type gt struct {
			count       int
			countUnique int
			day         string
			systemID    int64
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			if h.SystemID == 0 {
				_, h.SystemID = getUA(ctx, h.UserAgentID)
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := day + strconv.FormatInt(h.SystemID, 10)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.systemID = h.SystemID
				if !isReindex {
					var err error
					v.count, v.countUnique, err = existingSystemStats(ctx, tx,
						h.Site, day, h.SystemID)
					if err != nil {
						return err
					}
				}
			}

			v.count += 1
			if h.FirstVisit {
				v.countUnique += 1
			}
			grouped[k] = v
		}

		siteID := goatcounter.MustGetSite(ctx).ID
		ins := bulk.NewInsert(ctx, "system_stats", []string{"site_id", "day",
			"system_id", "count", "count_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.systemID, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

func existingSystemStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day string, systemID int64,
) (int, int, error) {

	var c []struct {
		Count       int `db:"count"`
		CountUnique int `db:"count_unique"`
	}
	err := tx.SelectContext(txctx, &c, `/* existingSystemStats */
		select count, count_unique from system_stats
		where site_id=$1 and day=$2 and system_id=$3 limit 1`,
		siteID, day, systemID)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from system_stats where
		site_id=$1 and day=$2 and system_id=$3`,
		siteID, day, systemID)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

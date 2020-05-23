// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package cron

import (
	"context"
	"fmt"

	"zgo.at/errors"
	"zgo.at/gadget"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Systems are stored as a count per system/version per day:
//
//  site |    day     | system  | version | count
// ------+------------+---------+---------+------
//     1 | 2019-12-17 | Chrome  | 38      |    13
//     1 | 2019-12-17 | Chrome  | 77      |     2
//     1 | 2019-12-17 | Opera   | 9       |     1
func updateSystemStats(ctx context.Context, hits []goatcounter.Hit) error {
	return zdb.TX(ctx, func(ctx context.Context, tx zdb.DB) error {
		// Group by day + system.
		type gt struct {
			count       int
			countUnique int
			day         string
			system      string
			version     string
		}
		grouped := map[string]gt{}
		for _, h := range hits {
			if h.Bot > 0 {
				continue
			}

			system, version := getSystem(h.Browser)
			if system == "" {
				continue
			}

			day := h.CreatedAt.Format("2006-01-02")
			k := fmt.Sprintf("%s%s%s", day, system, version)
			v := grouped[k]
			if v.count == 0 {
				v.day = day
				v.system = system
				v.version = version
				var err error
				v.count, v.countUnique, err = existingSystemStats(ctx, tx,
					h.Site, day, v.system, v.version)
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
		ins := bulk.NewInsert(ctx, "system_stats", []string{"site", "day",
			"system", "version", "count", "count_unique"})
		for _, v := range grouped {
			ins.Values(siteID, v.day, v.system, v.version, v.count, v.countUnique)
		}
		return ins.Finish()
	})
}

func existingSystemStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, system, version string,
) (int, int, error) {

	var c []struct {
		Count       int `db:"count"`
		CountUnique int `db:"count_unique"`
	}
	err := tx.SelectContext(txctx, &c, `/* existingSystemStats */
		select count, count_unique from system_stats
		where site=$1 and day=$2 and system=$3 and version=$4 limit 1`,
		siteID, day, system, version)
	if err != nil {
		return 0, 0, errors.Wrap(err, "select")
	}
	if len(c) == 0 {
		return 0, 0, nil
	}

	_, err = tx.ExecContext(txctx, `delete from system_stats where
		site=$1 and day=$2 and system=$3 and version=$4`,
		siteID, day, system, version)
	return c[0].Count, c[0].CountUnique, errors.Wrap(err, "delete")
}

func getSystem(uaHeader string) (string, string) {
	ua := gadget.Parse(uaHeader)
	return ua.OSName, ua.OSVersion
}

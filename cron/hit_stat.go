// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/utils/jsonutil"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
)

// Hit stats are stored per day/path, the value is a 2-tuple: it lists the
// counts for every hour. The first is the hour, second the number of hits in
// that hour:
//
//   site       | 1
//   day        | 2019-12-05
//   path       | /jquery.html
//   stats      | [[0,0],[1,2],[2,2],[3,0],[4,0],[5,0],[6,1],[7,2],[8,3],
//                 [9,0],[10,2],[11,2],[12,2],[13,5],[14,4],[15,3],[16,0],
//                 [17,1],[18,2],[19,0],[20,0],[21,1],[22,4],[23,2]]
//
// TODO: this can either just assume hour by index, or not store all the hours.
// TODO: need to fill in blank days.
func updateHitStats(ctx context.Context, phits map[string][]goatcounter.Hit) error {
	txctx, tx, err := zdb.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_ = txctx

	// Group by day + path.
	type gt struct {
		count [][]int
		day   string
		path  string
	}
	grouped := map[string]gt{}
	for _, hits := range phits {
		for _, h := range hits {
			day := h.CreatedAt.Format("2006-01-02")
			k := day + h.Path
			v := grouped[k]
			if len(v.count) == 0 {
				v.day = day
				v.path = h.Path

				// Append existing and delete from DB; this will be faster than
				// running an update for every row.
				var c []byte
				err := tx.GetContext(txctx, &c,
					`select stats from hit_stats where site=$1 and day=$2 and path=$3`,
					h.Site, day, v.path)
				if err != sql.ErrNoRows {
					if err != nil {
						return errors.Wrap(err, "existing")
					}
					_, err = tx.ExecContext(txctx,
						`delete from hit_stats where site=$1 and day=$2 and path=$3`,
						h.Site, day, v.path)
					if err != nil {
						return errors.Wrap(err, "delete")
					}
				}

				if c != nil {
					jsonutil.MustUnmarshal(c, &v.count)
				} else {
					v.count = make([][]int, 24)
					for i := range v.count {
						v.count[i] = []int{i, 0}
					}
				}
			}

			h, _ := strconv.ParseInt(h.CreatedAt.Format("15"), 10, 8)
			v.count[h][1] += 1
			grouped[k] = v
		}
	}

	siteID := goatcounter.MustGetSite(ctx).ID
	ins := bulk.NewInsert(ctx, zdb.MustGet(ctx),
		"hit_stats", []string{"site", "day", "path", "stats"})
	for _, v := range grouped {
		ins.Values(siteID, v.day, v.path, jsonutil.MustMarshal(v.count))
	}
	err = ins.Finish()
	if err != nil {
		return err
	}

	return tx.Commit()
}

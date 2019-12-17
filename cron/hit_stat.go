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
// TODO: need to fill in blank days.
// TODO: this can either just assume hour by index, or not store all the hours.
// TODO: rename "stats" to "hourly" and add a daily int count.
func updateHitStats(ctx context.Context, hits []goatcounter.Hit) error {
	txctx, tx, err := zdb.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Group by day + path.
	type gt struct {
		count [][]int
		day   string
		path  string
	}
	grouped := map[string]gt{}
	for _, h := range hits {
		day := h.CreatedAt.Format("2006-01-02")
		k := day + h.Path
		v := grouped[k]
		if len(v.count) == 0 {
			v.day = day
			v.path = h.Path
			v.count, err = existingHitStats(ctx, tx, h.Site, day, v.path)
			if err != nil {
				return err
			}
		}

		h, _ := strconv.ParseInt(h.CreatedAt.Format("15"), 10, 8)
		v.count[h][1] += 1
		grouped[k] = v
	}

	siteID := goatcounter.MustGetSite(ctx).ID
	ins := bulk.NewInsert(txctx, tx,
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

func existingHitStats(
	txctx context.Context, tx zdb.DB, siteID int64,
	day, path string,
) ([][]int, error) {

	var c []byte
	err := tx.GetContext(txctx, &c,
		`select stats from hit_stats where site=$1 and day=$2 and path=$3`,
		siteID, day, path)
	if err != nil && err != sql.ErrNoRows {
		return nil, errors.Wrap(err, "existing")
	}

	if err != sql.ErrNoRows {
		_, err = tx.ExecContext(txctx,
			`delete from hit_stats where site=$1 and day=$2 and path=$3`,
			siteID, day, path)
		if err != nil {
			return nil, errors.Wrap(err, "delete")
		}
	}

	var r [][]int
	if c != nil {
		jsonutil.MustUnmarshal(c, &r)
		return r, nil
	}

	r = make([][]int, 24)
	for i := range r {
		r[i] = []int{i, 0}
	}
	return r, nil
}

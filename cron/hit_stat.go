// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at eupl12.zgo.at

package cron

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/utils/jsonutil"
	"zgo.at/utils/sliceutil"
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
//   stats      | [0,0,0,0,0,0,0,0,0,0,0,4,7,0,0,0,0,0,0,0,0,0,1,0]
func updateHitStats(ctx context.Context, hits []goatcounter.Hit) error {
	txctx, tx, err := zdb.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Group by day + path.
	type gt struct {
		count []int
		total int
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
			for _, c := range v.count {
				v.total += c
			}
		}

		h, _ := strconv.ParseInt(h.CreatedAt.Format("15"), 10, 8)
		v.count[h] += 1
		v.total += 1
		grouped[k] = v
	}

	siteID := goatcounter.MustGetSite(ctx).ID
	ins := bulk.NewInsert(txctx, tx,
		"hit_stats", []string{"site", "day", "path", "stats", "total"})
	for _, v := range grouped {
		ins.Values(siteID, v.day, v.path, jsonutil.MustMarshal(v.count), v.total)
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
) ([]int, error) {

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

	var r []int
	if c != nil {
		jsonutil.MustUnmarshal(c, &r)
		return r, nil
	}

	r = make([]int, 24)
	for i := range r {
		r[i] = 0 // TODO: not needed?
	}
	return r, nil
}

const allDays = `[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]`

var ranDay int

// Every path must have a row for every day since the start of the site, even if
// there are no hits. This makes the SQL queries and chart generation a lot
// easier and faster later on, at the expense of storing more "useless" data.
func fillBlanksForToday(ctx context.Context) error {
	// Run once a day only (it's okay to run more than once, just a waste of
	// time).
	if time.Now().UTC().Day() == ranDay {
		return nil
	}
	ranDay = time.Now().UTC().Day()

	var allpaths []struct {
		Site int64
		Path string
	}
	err := zdb.MustGet(ctx).SelectContext(ctx, &allpaths,
		`select site, path from hits group by site, path`)
	if err != nil {
		return err
	}

	today := time.Now().UTC().Format("2006-01-02")

	var have []string
	err = zdb.MustGet(ctx).SelectContext(ctx, &have,
		`select site || path from hit_stats where day=$1`, today)
	if err != nil {
		return err
	}

	ins := bulk.NewInsert(ctx, zdb.MustGet(ctx),
		"hit_stats", []string{"site", "day", "path", "stats"})
	for _, p := range allpaths {
		if sliceutil.InStringSlice(have, fmt.Sprintf("%d%s", p.Site, p.Path)) {
			continue
		}

		ins.Values(p.Site, today, p.Path, allDays)
	}

	return ins.Finish()
}

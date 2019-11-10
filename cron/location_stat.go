// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zdb/bulk"
	"zgo.at/zhttp/ctxkey"
)

type lstat struct {
	Location  string    `db:"location"`
	Count     int       `db:"count"`
	CreatedAt time.Time `db:"created_at"`
}

func updateLocationStats(ctx context.Context, site goatcounter.Site) error {
	ctx = context.WithValue(ctx, ctxkey.Site, &site)
	db := zdb.MustGet(ctx)

	// Select everything since last update.
	var last string
	if site.LastStat == nil {
		last = "1970-01-01"
	} else {
		last = site.LastStat.Format("2006-01-02")
	}

	var query string
	if cfg.PgSQL {
		query = `
			select
				location,
				count(location) as count,
				cast(substr(cast(created_at as varchar), 0, 14) || ':00:00' as timestamp) as created_at
			from hits
			where
				site=$1 and
				created_at>=$2
			group by location, substr(cast(created_at as varchar), 0, 14)
			order by count desc`
	} else {
		query = `
			select
				location,
				count(location) as count,
				created_at
			from hits
			where
				site=$1 and
				created_at>=$2
			group by location, strftime('%Y-%m-%d %H', created_at)
			order by count desc`
	}

	var stats []lstat
	err := db.SelectContext(ctx, &stats, query, site.ID, last)
	if err != nil {
		return errors.Wrap(err, "fetch data")
	}

	// Remove everything we'll update; it's faster than running many updates.
	_, err = db.ExecContext(ctx, `delete from location_stats where site=$1 and day>=$2`,
		site.ID, last)
	if err != nil {
		return errors.Wrap(err, "delete")
	}

	// Group properly.
	type gt struct {
		count    int
		day      string
		location string
	}
	grouped := map[string]gt{}
	for _, s := range stats {
		k := s.CreatedAt.Format("2006-01-02") + s.Location
		v := grouped[k]
		if v.count == 0 {
			v.day = s.CreatedAt.Format("2006-01-02")
			v.location = s.Location
		}

		v.count += s.Count
		grouped[k] = v
	}

	insLocation := bulk.NewInsert(ctx, zdb.MustGet(ctx).(*sqlx.DB),
		"location_stats", []string{"site", "day", "location", "count"})
	for _, v := range grouped {
		insLocation.Values(site.ID, v.day, v.location, v.count)
	}

	return insLocation.Finish()
}

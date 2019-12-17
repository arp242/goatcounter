// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

// Package cron schedules jobs.
package cron

import (
	"context"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/utils/syncutil"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

type task struct {
	fun    func(context.Context) error
	period time.Duration
}

var tasks = []task{
	{persistAndStat, 3 * time.Second},
}

var stopped = syncutil.NewAtomicInt(0)

var wg sync.WaitGroup

// Run stat updates in the background.
func Run(db *sqlx.DB) {
	ctx := zdb.With(context.Background(), db)
	l := zlog.Module("cron")

	for _, t := range tasks {
		// Run everything on startup immediately.
		err := t.fun(ctx)
		if err != nil {
			l.Error(err)
		}

		go func(t task) {
			defer zlog.Recover()

			for {
				time.Sleep(t.period)
				if stopped.Value() == 1 {
					return
				}

				var err error
				func() {
					wg.Add(1)
					defer wg.Done()
					err = t.fun(ctx)
				}()
				if err != nil {
					l.Error(err)
				}
			}
		}(t)
	}
}

// Wait for all running tasks to finish and then run all tasks for consistency
// on shutdown.
func Wait(db *sqlx.DB) {
	stopped.Set(1)
	ctx := zdb.With(context.Background(), db)

	wg.Wait()

	for _, t := range tasks {
		err := t.fun(ctx)
		if err != nil {
			zlog.Module("cron").Error(err)
		}
	}
}

func persistAndStat(ctx context.Context) error {
	l := zlog.Module("cron")

	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		return err
	}
	l = l.Since("memstore")

	err = updateStats(ctx, hits)
	if len(hits) > 0 {
		l.Since("stats").FieldsSince().Printf("persisted %d hits", len(hits))
	}
	return err
}

func updateStats(ctx context.Context, hits []goatcounter.Hit) error {
	// Group by site and path.
	grouped := make(map[int64]map[string][]goatcounter.Hit)
	for _, h := range hits {
		_, ok := grouped[h.Site]
		if !ok {
			grouped[h.Site] = make(map[string][]goatcounter.Hit)
		}

		grouped[h.Site][h.Path] = append(grouped[h.Site][h.Path], h)
	}

	for siteID, paths := range grouped {
		start := time.Now().UTC().Format("2006-01-02 15:04:05")
		var site goatcounter.Site
		err := site.ByID(ctx, siteID)
		if err != nil {
			return err
		}
		ctx = context.WithValue(ctx, ctxkey.Site, &site)

		err = updateHitStats(ctx, paths)
		if err != nil {
			return errors.Wrapf(err, "hit_stat: site %d", siteID)
		}
		err = updateBrowserStats(ctx, paths)
		if err != nil {
			return errors.Wrapf(err, "browser_stat: site %d", siteID)
		}
		err = updateLocationStats(ctx, paths)
		if err != nil {
			return errors.Wrapf(err, "location_stat: site %d", siteID)
		}

		// Record last update.
		_, err = zdb.MustGet(ctx).ExecContext(ctx,
			`update sites set last_stat=$1 where id=$2`, start, siteID)
		if err != nil {
			return errors.Wrapf(err, "update last_stat: site %d", siteID)
		}
		if !site.ReceivedData {
			_, err = zdb.MustGet(ctx).ExecContext(ctx,
				`update sites set received_data=1 where id=$1`, siteID)
			if err != nil {
				return errors.Wrapf(err, "update received_data: site %d", siteID)
			}
		}
	}

	return nil
}

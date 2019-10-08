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
	"zgo.at/zdb"
	"zgo.at/zlog"
)

type task struct {
	fun    func(context.Context) error
	period time.Duration
}

var tasks = []task{
	{persistAndStat, 10 * time.Second},
}

var wg sync.WaitGroup

// Run stat updates in the background.
//
// TODO: If a cron job takes longer than the period it might get run twice. Not
// sure if we want that.
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

	hl := goatcounter.Memstore.Len()
	err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		return err
	}
	l = l.Since("memstore")

	err = updateStats(ctx)
	if hl > 0 {
		l.Since("stats").FieldsSince().Printf("persisted %d hits", hl)
	}
	return err
}

func updateStats(ctx context.Context) error {
	var sites goatcounter.Sites
	err := sites.List(ctx)
	if err != nil {
		return err
	}

	for _, s := range sites {
		start := time.Now().Format("2006-01-02 15:04:05")

		err := updateHitStats(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "hit_stat: site %d", s.ID)
		}

		err = updateBrowserStats(ctx, s)
		if err != nil {
			return errors.Wrapf(err, "browser_stat: site %d", s.ID)
		}

		// Record last update.
		_, err = zdb.MustGet(ctx).ExecContext(ctx,
			`update sites set last_stat=$1, received_data=1 where id=$2`,
			start, s.ID)
		if err != nil {
			return errors.Wrapf(err, "update last_stat: site %d", s.ID)
		}
	}
	return nil
}

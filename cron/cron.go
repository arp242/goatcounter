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
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"

	"zgo.at/goatcounter"
)

type task struct {
	fun    func(context.Context) error
	period time.Duration
}

var tasks = []task{
	{goatcounter.Memstore.Persist, 10 * time.Second},
	{updateAllHitStats, 60 * time.Second},
}

var wg sync.WaitGroup

// Run stat updates in the background.
//
// TODO: If a cron job takes longer than the period it might get run twice. Not
// sure if we want that.
func Run(db *sqlx.DB) {
	ctx := context.WithValue(context.Background(), ctxkey.DB, db)
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
	ctx := context.WithValue(context.Background(), ctxkey.DB, db)
	l := zlog.Module("cron")

	wg.Wait()

	for _, t := range tasks {
		err := t.fun(ctx)
		if err != nil {
			l.Error(err)
		}
	}
}

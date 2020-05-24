// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// Package cron schedules jobs.
package cron

import (
	"context"
	"sync"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zsync"
)

type task struct {
	fun    func(context.Context) error
	period time.Duration
}

var tasks = []task{
	{persistAndStat, 10 * time.Second},
	{DataRetention, 1 * time.Hour},
	{renewACME, 2 * time.Hour},
	{vacuumDeleted, 12 * time.Hour},
	{goatcounter.Salts.Refresh, 1 * time.Hour},
	{clearSessions, 1 * time.Minute},
	{oldExports, 1 * time.Hour},
	{emailReports, 1 * time.Hour},
}

var (
	stopped = zsync.NewAtomicInt(0)
	wg      sync.WaitGroup
)

// RunOnce runs all tasks once and returns.
func RunOnce(db zdb.DB) {
	ctx := zdb.With(context.Background(), db)
	l := zlog.Module("cron")
	for _, t := range tasks {
		err := t.fun(ctx)
		if err != nil {
			l.Error(err)
		}
	}
}

// RunBackground runs tasks in the background according to the given schedule.
func RunBackground(db zdb.DB) {
	ctx := zdb.With(context.Background(), db)
	l := zlog.Module("cron")

	for _, t := range tasks {
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
func Wait(db zdb.DB) {
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

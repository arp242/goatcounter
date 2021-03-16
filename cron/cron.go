// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package cron schedules jobs.
package cron

import (
	"context"
	"strings"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/zlog"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zsync"
)

type task struct {
	fun    func(context.Context) error
	period time.Duration
}

var tasks = []task{
	{PersistAndStat, 10 * time.Second},
	{DataRetention, 1 * time.Hour},
	{renewACME, 2 * time.Hour},
	{vacuumDeleted, 12 * time.Hour},
	{cancelPlan, 12 * time.Hour},
	{oldExports, 1 * time.Hour},
	{sessions, 1 * time.Minute},
}

var stopped = zsync.NewAtomicInt(0)

// RunBackground runs tasks in the background according to the given schedule.
func RunBackground(ctx context.Context) {
	l := zlog.Module("cron")

	// TODO: should rewrite cron to always respond to channels, and then have
	// the cron package send those periodically.
	go func() {
		for {
			<-goatcounter.PersistRunner.Run
			bgrun.RunNoDuplicates("cron:PersistAndStat", func() {
				err := PersistAndStat(ctx)
				if err != nil {
					l.Error(err)
				}
			})
		}
	}()

	for _, t := range tasks {
		go func(t task) {
			defer zlog.Recover()

			for {
				time.Sleep(t.period)
				if stopped.Value() == 1 {
					return
				}

				f := strings.Replace(zruntime.FuncName(t.fun), "zgo.at/goatcounter/cron.", "", 1)
				bgrun.RunNoDuplicates("cron:"+f, func() {
					err := t.fun(ctx)
					if err != nil {
						l.Error(err)
					}
				})
			}
		}(t)
	}
}

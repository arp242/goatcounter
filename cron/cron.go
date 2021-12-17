// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package cron schedules jobs.
package cron

import (
	"context"
	"strings"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/bgrun"
	"zgo.at/zlog"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zsync"
)

type Task struct {
	Desc   string
	Fun    func(context.Context) error
	Period time.Duration
}

func (t Task) ID() string {
	return strings.Replace(zruntime.FuncName(t.Fun), "zgo.at/goatcounter/v2/cron.", "", 1)
}

var Tasks = []Task{
	{"persist hits", PersistAndStat, 10 * time.Second},
	{"vacuum pageviews (data retention)", DataRetention, 1 * time.Hour},
	{"renew ACME certs", renewACME, 2 * time.Hour},
	{"vacuum soft-deleted sites", vacuumDeleted, 12 * time.Hour},
	{"process scheduled plan changes", cancelPlan, 12 * time.Hour},
	{"rm old exports", oldExports, 1 * time.Hour},
	{"cycle sessions", sessions, 1 * time.Minute},
	{"report usage", reportUsage, 12 * time.Hour},
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
				done := timeout("PersistAndStat", 10*time.Second)
				err := PersistAndStat(ctx)
				if err != nil {
					l.Error(err)
				}
				done <- struct{}{}
			})
		}
	}()

	for _, t := range Tasks {
		go func(t Task) {
			defer zlog.Recover()

			for {
				time.Sleep(t.Period)
				if stopped.Value() == 1 {
					return
				}

				f := t.ID()
				bgrun.RunNoDuplicates("cron:"+f, func() {
					done := timeout(f, 10*time.Second)
					err := t.Fun(ctx)
					if err != nil {
						l.Error(err)
					}
					done <- struct{}{}
				})
			}
		}(t)
	}
}

func timeout(f string, d time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		t := time.NewTimer(d)
		select {
		case <-t.C:
			zlog.Errorf("cron task %s is taking longer than %s", f, d)
		case <-done:
			t.Stop()
		}
	}()
	return done
}

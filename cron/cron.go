// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

// Package cron schedules jobs.
package cron

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"zgo.at/bgrun"
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
	{"vacuum pageviews (data retention)", dataRetention, 1 * time.Hour},
	{"vacuum pageviews (old bot)", oldBot, 1 * time.Hour},
	{"renew ACME certs", renewACME, 2 * time.Hour},
	{"vacuum soft-deleted sites", vacuumDeleted, 12 * time.Hour},
	{"rm old exports", oldExports, 1 * time.Hour},
	{"cycle sessions", sessions, 1 * time.Minute},
	{"send email reports", emailReports, 1 * time.Hour},
	{"persist hits", persistAndStat, time.Duration(persistInterval.Load())},
}

var (
	stopped         = zsync.NewAtomicInt(0)
	started         = zsync.NewAtomicInt(0)
	persistInterval = func() atomic.Int64 {
		var d atomic.Int64
		d.Store(int64(10 * time.Second))
		return d
	}()
)

func SetPersistInterval(d time.Duration) {
	persistInterval.Store(int64(d))
}

// Start running tasks in the background.
func Start(ctx context.Context) {
	if started.Value() == 1 {
		return
	}
	started.Set(1)

	l := zlog.Module("cron")

	for _, t := range Tasks {
		t := t
		f := t.ID()
		bgrun.NewTask("cron:"+f, 1, func(context.Context) error {
			err := t.Fun(ctx)
			if err != nil {
				l.Error(err)
			}
			return nil
		})
	}

	for _, t := range Tasks {
		go func(t Task) {
			defer zlog.Recover()
			id := t.ID()

			for {
				if id == "persistAndStat" {
					time.Sleep(time.Duration(persistInterval.Load()))
				} else {
					time.Sleep(t.Period)
				}
				if stopped.Value() == 1 {
					return
				}

				err := bgrun.RunTask("cron:" + id)
				if err != nil {
					zlog.Error(err)
				}
			}
		}(t)
	}
}

func Stop() error {
	stopped.Set(1)
	started.Set(0)
	bgrun.Wait("")
	bgrun.Reset()
	return nil
}

func TaskOldExports() error     { return bgrun.RunTask("cron:oldExports") }
func TaskDataRetention() error  { return bgrun.RunTask("cron:dataRetention") }
func TaskVacuumOldSites() error { return bgrun.RunTask("cron:vacuumDeleted") }
func TaskACME() error           { return bgrun.RunTask("cron:renewACME") }
func TaskSessions() error       { return bgrun.RunTask("cron:sessions") }
func TaskEmailReports() error   { return bgrun.RunTask("cron:emailReports") }
func TaskPersistAndStat() error { return bgrun.RunTask("cron:persistAndStat") }
func WaitOldExports()           { bgrun.Wait("cron:oldExports") }
func WaitDataRetention()        { bgrun.Wait("cron:dataRetention") }
func WaitVacuumOldSites()       { bgrun.Wait("cron:vacuumDeleted") }
func WaitACME()                 { bgrun.Wait("cron:renewACME") }
func WaitSessions()             { bgrun.Wait("cron:sessions") }
func WaitEmailReports()         { bgrun.Wait("cron:emailReports") }
func WaitPersistAndStat()       { bgrun.Wait("cron:persistAndStat") }

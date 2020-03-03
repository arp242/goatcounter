// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// Package cron schedules jobs.
package cron

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
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
	{persistAndStat, 10 * time.Second},
	{DataRetention, 1 * time.Hour},
	{renewACME, 2 * time.Hour},
}

var (
	stopped = syncutil.NewAtomicInt(0)
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

func DataRetention(ctx context.Context) error {
	var sites goatcounter.Sites
	err := sites.List(ctx)
	if err != nil {
		return err
	}

	for _, s := range sites {
		if s.Settings.DataRetention <= 0 {
			continue
		}

		err = s.DeleteOlderThan(ctx, s.Settings.DataRetention)
		if err != nil {
			zlog.Module("cron").Field("site", s.ID).Error(err)
		}
	}

	return nil
}

func persistAndStat(ctx context.Context) error {
	l := zlog.Module("cron")

	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		return err
	}
	if len(hits) > 0 {
		l = l.Since("memstore")
	}

	grouped := make(map[int64][]goatcounter.Hit)
	for _, h := range hits {
		grouped[h.Site] = append(grouped[h.Site], h)
	}
	for siteID, hits := range grouped {
		err := UpdateStats(ctx, siteID, hits)
		if err != nil {
			l.Fields(zlog.F{
				"site":  siteID,
				"paths": hits,
			}).Error(err)
		}
	}

	if len(hits) > 100 {
		l.Since("stats").FieldsSince().Printf("persisted %d hits", len(hits))
	}
	return err
}

func UpdateStats(ctx context.Context, siteID int64, hits []goatcounter.Hit) error {
	start := time.Now().UTC().Format("2006-01-02 15:04:05")
	var site goatcounter.Site
	err := site.ByID(ctx, siteID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, ctxkey.Site, &site)

	err = updateHitStats(ctx, hits)
	if err != nil {
		return errors.Wrapf(err, "hit_stat: site %d", siteID)
	}
	err = updateBrowserStats(ctx, hits)
	if err != nil {
		return errors.Wrapf(err, "browser_stat: site %d", siteID)
	}
	err = updateLocationStats(ctx, hits)
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
	return nil
}

func ReindexStats(ctx context.Context, hits []goatcounter.Hit) error {
	grouped := make(map[int64][]goatcounter.Hit)
	for _, h := range hits {
		grouped[h.Site] = append(grouped[h.Site], h)
	}

	for siteID, hits := range grouped {
		err := UpdateStats(ctx, siteID, hits)
		if err != nil {
			return err
		}
	}

	return nil
}

func renewACME(ctx context.Context) error {
	if !acme.Enabled() {
		return nil
	}

	// Don't do this on shutdown as the HTTP server won't be available.
	if stopped.Value() == 1 {
		return nil
	}

	var sites goatcounter.Sites
	err := sites.ListCnames(ctx)
	if err != nil {
		return err
	}

	for _, s := range sites {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			err := acme.Make(d)
			if err != nil {
				zlog.Module("cron-acme").Error(err)
			}
		}(*s.Cname)
	}

	return nil
}

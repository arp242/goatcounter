// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package cron

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/acme"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/ztime"
)

func oldExports(ctx context.Context) error {
	tmp := os.TempDir()
	d, err := os.Open(tmp)
	if err != nil {
		return errors.Errorf("cron.oldExports: %w", err)
	}

	files, err := d.Readdirnames(-1)
	if err != nil {
		return errors.Errorf("cron.oldExports: %w", err)
	}

	tmp += "/"
	for _, f := range files {
		if !strings.HasPrefix(f, "goatcounter-export-") {
			continue
		}

		f = tmp + f
		st, err := os.Stat(f)
		if err != nil {
			zlog.Errorf("cron.oldExports: %s", err)
			continue
		}

		if st.ModTime().Before(ztime.Now().Add(-24 * time.Hour)) {
			err := os.Remove(f)
			if err != nil {
				zlog.Errorf("cron.oldExports: %s", err)
			}
		}
	}

	return nil
}

func dataRetention(ctx context.Context) error {
	var sites goatcounter.Sites
	err := sites.UnscopedList(ctx)
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

func oldBot(ctx context.Context) error {
	ival := goatcounter.Interval(ctx, 30)
	err := zdb.Exec(ctx, `delete from hits where bot > 0 and created_at < `+ival)
	if err != nil {
		zlog.Module("cron").Error(err)
	}
	return nil
}

func persistAndStat(ctx context.Context) error {
	l := zlog.Module("cron")
	l.Debug("persistAndStat started")

	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		return err
	}
	if len(hits) > 0 {
		l = l.Since("memstore")
	}

	grouped := make(map[int64][]goatcounter.Hit)
	for _, h := range hits {
		if h.Bot > 0 {
			continue
		}
		grouped[h.Site] = append(grouped[h.Site], h)
	}
	for siteID, hits := range grouped {
		err := UpdateStats(ctx, nil, siteID, hits)
		if err != nil {
			l.Fields(zlog.F{
				"site":  siteID,
				"paths": hits,
			}).Error(err)
		}
	}

	if len(hits) > 0 {
		l.Since("stats").FieldsSince().Debugf("persisted %d hits", len(hits))
	}
	return err
}

// UpdateStats updates all the stats tables.
//
// Exported for tests.
func UpdateStats(ctx context.Context, site *goatcounter.Site, siteID int64, hits []goatcounter.Hit) error {
	if site == nil {
		site = new(goatcounter.Site)
		err := site.ByID(ctx, siteID)
		if err != nil {
			return err
		}
	}
	ctx = goatcounter.WithSite(ctx, site)

	funs := []func(context.Context, []goatcounter.Hit) error{
		updateHitCounts,
		updateRefCounts,
		updateHitStats,
		updateBrowserStats,
		updateSystemStats,
		updateLocationStats,
		updateLanguageStats,
		updateSizeStats,
		updateCampaignStats,
	}

	for _, f := range funs {
		err := f(ctx, hits)
		if err != nil {
			return errors.Wrapf(err, "site %d", siteID)
		}
	}

	if !site.ReceivedData {
		err := site.UpdateReceivedData(ctx)
		if err != nil {
			return errors.Wrapf(err, "update received_data: site %d", siteID)
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
	err := sites.UnscopedListCnames(ctx)
	if err != nil {
		return err
	}

	for _, s := range sites {
		err := acme.Make(ctx, *s.Cname)
		if err != nil {
			zlog.Module("cron-acme").Field("cname", *s.Cname).Error(err)
			continue
		}

		err = s.UpdateCnameSetupAt(ctx)
		if err != nil {
			zlog.Module("cron-acme").Field("cname", *s.Cname).Error(err)
			continue
		}
	}

	return nil
}

// Permanently delete soft-deleted sites.
func vacuumDeleted(ctx context.Context) error {
	var sites goatcounter.Sites
	err := sites.OldSoftDeleted(ctx)
	if err != nil {
		return errors.Errorf("vacuumDeleted: %w", err)
	}

	for _, s := range sites {
		zlog.Module("vacuum").Printf("vacuum site %s/%d", s.Code, s.ID)
		err := zdb.TX(ctx, func(ctx context.Context) error {
			for _, t := range []string{"hits", "paths",
				"hit_counts", "ref_counts",
				"browser_stats", "system_stats", "hit_stats", "location_stats", "language_stats", "size_stats",
				"campaign_stats", "exports", "api_tokens", "users", "sites"} {

				err := zdb.Exec(ctx, fmt.Sprintf(`delete from %s where site_id=%d`, t, s.ID))
				if err != nil {
					return errors.Errorf("%s: %w", t, err)
				}
			}
			return nil
		})
		if err != nil {
			return errors.Errorf("vacuumDeleted: %w", err)
		}
	}
	return nil
}

func sessions(ctx context.Context) error {
	goatcounter.Memstore.EvictSessions()
	return nil
}

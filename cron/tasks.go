package cron

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/acme"
	"zgo.at/goatcounter/v2/log"
	"zgo.at/zdb"
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
			log.Errorf(ctx, "cron.oldExports: %s", err)
			continue
		}

		if st.ModTime().Before(ztime.Now().Add(-24 * time.Hour)) {
			err := os.Remove(f)
			if err != nil {
				log.Errorf(ctx, "cron.oldExports: %s", err)
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
			log.Module("cron").Error(ctx, err, "site", s.ID)
		}
	}

	return nil
}

func oldBot(ctx context.Context) error {
	ival := goatcounter.Interval(ctx, 30)
	err := zdb.Exec(ctx, `delete from hits where bot > 0 and created_at < `+ival)
	if err != nil {
		log.Module("cron").Error(ctx, err)
	}
	return nil
}

func persistAndStat(ctx context.Context) error {
	l := log.Module("cron")
	l.Debug(ctx, "persistAndStat started")

	start := ztime.Now()
	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		return err
	}

	var (
		startStats = ztime.Now()
		grouped    = make(map[int64][]goatcounter.Hit)
	)
	for _, h := range hits {
		if h.Bot > 0 {
			continue
		}
		grouped[h.Site] = append(grouped[h.Site], h)
	}
	for siteID, hits := range grouped {
		err := UpdateStats(ctx, nil, siteID, hits)
		if err != nil {
			l.Error(ctx, err, "site", siteID, "paths", hits)
		}
	}

	if len(hits) > 0 {
		l.Debug(ctx, "persisted hits",
			"num", len(hits),
			slog.Group("took",
				"memstore", time.Since(start).Round(time.Millisecond),
				"stats", time.Since(startStats).Round(time.Millisecond),
			))
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
			log.Module("cron-acme").Error(ctx, err, "cname", *s.Cname)
			continue
		}

		err = s.UpdateCnameSetupAt(ctx)
		if err != nil {
			log.Module("cron-acme").Error(ctx, err, "cname", *s.Cname)
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
		log.Module("vacuum").Infof(ctx, "vacuum site %s/%d", s.Code, s.ID)
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

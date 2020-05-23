// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
)

type AdminStat struct {
	ID         int64     `db:"id"`
	Parent     *int64    `db:"parent"`
	Code       string    `db:"code"`
	Stripe     *string   `db:"stripe"`
	LinkDomain string    `db:"link_domain"`
	CreatedAt  time.Time `db:"created_at"`
	Plan       string    `db:"plan"`
	LastMonth  int       `db:"last_month"`
	Total      int       `db:"total"`
}

type AdminStats []AdminStat

// List stats for all sites, for all time.
func (a *AdminStats) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			sites.id,
			sites.parent,
			sites.code,
			sites.created_at,
			(case
				when sites.stripe is null then 'free'
				when substr(sites.stripe, 0, 9) = 'cus_free' then 'free'
				else sites.plan
			end) as plan,
			stripe,
			sites.link_domain,
			coalesce((
				select sum(hit_counts.total) from hit_counts where site=sites.id
			), 0) as total,
			coalesce((
				select sum(hit_counts.total) from hit_counts
				where site=sites.id and hit_counts.hour >= %s
			), 0) as last_month
		from sites
		group by sites.id, sites.code, sites.created_at, plan
		order by last_month desc`, interval(30)))
	if err != nil {
		return errors.Wrap(err, "AdminStats.List")
	}

	// Add all the child plan counts to the parents.
	aa := *a
	for _, s := range aa {
		if s.Plan != PlanChild {
			continue
		}

		for i, s2 := range aa {
			if s2.ID == *s.Parent {
				aa[i].Total += s.Total
				aa[i].LastMonth += s.LastMonth
				break
			}
		}
	}
	sort.Slice(aa, func(i, j int) bool { return aa[i].LastMonth > aa[j].LastMonth })
	return nil
}

type AdminSiteStat struct {
	Site           Site      `db:"-"`
	User           User      `db:"-"`
	LastData       time.Time `db:"last_data"`
	CountTotal     int       `db:"count_total"`
	CountLastMonth int       `db:"count_last_month"`
	CountPrevMonth int       `db:"count_prev_month"`
}

// ByID gets stats for a single site.
func (a *AdminSiteStat) ByID(ctx context.Context, id int64) error {
	err := a.Site.ByID(ctx, id)
	if err != nil {
		return err
	}

	err = a.User.BySite(ctx, id)
	if err != nil {
		return err
	}

	ival30 := interval(30)
	ival60 := interval(30)
	err = zdb.MustGet(ctx).GetContext(ctx, a, fmt.Sprintf(`
		select
			coalesce((select hour from hit_counts where site=$1 order by hour desc limit 1), '1970-01-01') as last_data,
			coalesce((select sum(total) from hit_counts where site=$1), 0) as count_total,
			coalesce((select sum(total) from hit_counts where site=$1
				and hour >= %[1]s), 0) as count_last_month,
			coalesce((select sum(total) from hit_counts where site=$1
				and hour >= %[2]s
				and hour <= %[1]s
			), 0) as count_prev_month
		`, ival30, ival60), id)
	return errors.Wrap(err, "AdminSiteStats.ByID")
}

// ByCode gets stats for a single site.
func (a *AdminSiteStat) ByCode(ctx context.Context, code string) error {
	err := a.Site.ByHost(ctx, code+"."+cfg.Domain)
	if err != nil {
		return err
	}
	return a.ByID(ctx, a.Site.ID)
}

type AdminPgStatActivity []struct {
	PID      int64  `db:"pid"`
	Duration string `db:"duration"`
	Query    string `db:"query"`
}

func (a *AdminPgStatActivity) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
			pid,
			now() - pg_stat_activity.query_start as duration,
			query
		from pg_stat_activity
		where state != 'idle' and query not like '%from pg_stat_activity%';
	`)
	if err != nil {
		return fmt.Errorf("AdminPgActivity.List: %w", err)
	}

	aa := *a
	for i := range aa {
		aa[i].Query = normalizeQueryIndent(aa[i].Query)
	}

	*a = aa
	return nil
}

type AdminPgStatStatements []struct {
	Total    float64 `db:"total"`
	MeanTime float64 `db:"mean_time"`
	MinTime  float64 `db:"min_time"`
	MaxTime  float64 `db:"max_time"`
	Calls    int     `db:"calls"`
	QueryID  int64   `db:"queryid"`
	Query    string  `db:"query"`
}

func (a *AdminPgStatStatements) List(ctx context.Context, order, filter string) error {
	if order == "" {
		order = "total"
	}

	var (
		args  []interface{}
		where string
	)
	if filter != "" {
		args = append(args, "%"+filter+"%")
		where = `and query like $1`
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			(total_time / 1000 / 60) as total,
			mean_time,
			min_time,
			max_time,
			calls,
			queryid,
			query
		from pg_stat_statements where
			userid = (select usesysid from pg_user where usename = CURRENT_USER) and
			calls > 20 and
			query !~* '(^ *(copy|create|alter|explain) |from (pg_stat_|pg_catalog))'
			%s
		order by %s desc
		limit 100
	`, where, order), args...)
	if err != nil {
		return fmt.Errorf("AdminPgStatStatements.List: %w", err)
	}

	aa := *a
	for i := range aa {
		aa[i].Query = normalizeQueryIndent(aa[i].Query)
	}
	*a = aa
	return nil
}

type AdminPgStatTables []struct {
	Table   string `db:"relname"`
	SeqScan int64  `db:"seq_scan"`
	IdxScan int64  `db:"idx_scan"`
	SeqRead int64  `db:"seq_tup_read"`
	IdxRead int64  `db:"idx_tup_fetch"`

	LastVacuum      time.Time `db:"last_vacuum"`
	LastAutoVacuum  time.Time `db:"last_autovacuum"`
	LastAnalyze     time.Time `db:"last_analyze"`
	LastAutoAnalyze time.Time `db:"last_autoanalyze"`

	VacuumCount  int `db:"vacuum_count"`
	AnalyzeCount int `db:"analyze_count"`

	LiveTup         int64 `db:"n_live_tup"`
	DeadTup         int64 `db:"n_dead_tup"`
	ModSinceAnalyze int64 `db:"n_mod_since_analyze"`

	TableSize   int `db:"table_size"`
	IndexesSize int `db:"indexes_size"`
}

func (a *AdminPgStatTables) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
			relname,

			coalesce(seq_scan, 0) as seq_scan,
			coalesce(seq_tup_read, 0) as seq_tup_read,
			coalesce(idx_scan, 0) as idx_scan,
			coalesce(idx_tup_fetch, 0) as idx_tup_fetch,

			date(coalesce(last_vacuum,      now() - interval '50 year')) as last_vacuum,
			date(coalesce(last_autovacuum,  now() - interval '50 year')) as last_autovacuum,
			date(coalesce(last_analyze,     now() - interval '50 year')) as last_analyze,
			date(coalesce(last_autoanalyze, now() - interval '50 year')) as last_autoanalyze,

			vacuum_count  + autovacuum_count  as vacuum_count,
			analyze_count + autoanalyze_count as analyze_count,

			n_live_tup,
			n_dead_tup,
			n_mod_since_analyze,

			pg_table_size(  '"public"."' || relname || '"') / 1024/1024 as table_size,
			pg_indexes_size('"public"."' || relname || '"') / 1024/1024 as indexes_size

		from pg_stat_user_tables
		order by n_dead_tup
			/(n_live_tup
			* current_setting('autovacuum_vacuum_scale_factor')::float8
			+ current_setting('autovacuum_vacuum_threshold')::float8)
			desc
	`)
	return errors.Wrap(err, "AdminPgStatTables.List")
}

type AdminPgStatIndexes []struct {
	Table    string `db:"relname"`
	Size     int    `db:"size"`
	Index    string `db:"indexrelname"`
	Scan     int64  `db:"idx_scan"`
	TupRead  int64  `db:"idx_tup_read"`
	TupFetch int64  `db:"idx_tup_fetch"`
}

func (a *AdminPgStatIndexes) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
			relname,
			pg_relation_size('"public"."' || indexrelname || '"') / 1024/1024 as size,
			indexrelname,
			idx_scan,
			idx_tup_read,
			idx_tup_fetch
		from pg_stat_user_indexes
		order by idx_scan desc
	`)
	return errors.Wrap(err, "AdminPgStatTables.List")
}

// Normalize the indent a bit, because there are often of extra tabs inside
// Go literal strings.
func normalizeQueryIndent(q string) string {
	lines := strings.Split(q, "\n")
	if len(lines) < 2 {
		return strings.TrimSpace(q)
	}

	var n int
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(l), "/*") {
			continue
		}
		n = strings.Count(lines[1], "\t") - 1
		break
	}

	for j := range lines {
		lines[j] = strings.Replace(lines[j], "\t", "", n)
	}
	return strings.Join(lines, "\n")
}

type AdminPgStatProgress []struct {
	Table   string `db:"relname"`
	Command string `db:"command"`
	Phase   string `db:"phase"`
	Status  string `db:"status"`
}

func (a *AdminPgStatProgress) List(ctx context.Context) error {
	// https://www.postgresql.org/docs/current/progress-reporting.html
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
			relname,
			phase,
			command,
				'lockers: '    || lockers_done    || '/' || lockers_total    || '; ' ||
				'blocks: '     || blocks_done     || '/' || blocks_total     || '; ' ||
				'tuples: '     || tuples_done     || '/' || tuples_total     || '; ' ||
				'partitions: ' || partitions_done || '/' || partitions_total
			as status
		from pg_stat_progress_create_index
		join pg_stat_all_tables using(relid)

		union select
			relname,
			phase,
			'VACUUM' as command,
				'heap_blks: '          || heap_blks_total    || ', ' || heap_blks_scanned || ', ' || heap_blks_vacuumed || '; ' ||
				'index_vacuum_count: ' || index_vacuum_count || '; ' ||
				'max_dead_tuples: '    || max_dead_tuples    || '; ' ||
				'num_dead_tuples: '    || num_dead_tuples
			as status
		from pg_stat_progress_vacuum
		join pg_stat_all_tables using(relid)

		union select
			relname,
			phase,
			'VACUUM FULL' as command,
				'cluster_index_relid: ' || cluster_index_relid || '; ' ||
				'heap_tuples: '         || heap_tuples_scanned || '/'  || heap_tuples_written || '; ' ||
				'heap_blks: '           || heap_blks_scanned   || '/'  || heap_blks_total     || '; ' ||
				'index_rebuild_count: ' || index_rebuild_count
			as status
		from pg_stat_progress_cluster
		join pg_stat_all_tables using(relid)

	`)

	return errors.Wrap(err, "AdminPgStatProgress.List")

}

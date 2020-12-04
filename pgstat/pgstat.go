// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package pgstat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

type TableStat []struct {
	AttName     string  `db:"attname"`
	NullFrac    float64 `db:"null_frac"`
	AvgWidth    int     `db:"avg_width"`
	NDistinct   float64 `db:"n_distinct"`
	Correlation float64 `db:"correlation"`
}

func (a *TableStat) List(ctx context.Context, table string) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
		attname, coalesce(null_frac, 0) as null_frac, avg_width, n_distinct, correlation
		from pg_stats
		where schemaname='public' and tablename=$1`,
		table)
	return errors.Wrap(err, "TableStat.List")
}

type Activity []struct {
	PID      int64  `db:"pid"`
	Duration string `db:"duration"`
	Query    string `db:"query"`
}

func (a *Activity) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
			pid,
			now() - pg_stat_activity.query_start as duration,
			query
		from pg_stat_activity
		where state != 'idle' and query not like '%from pg_stat_activity%';
	`)
	if err != nil {
		return errors.Errorf("PgActivity.List: %w", err)
	}

	aa := *a
	for i := range aa {
		aa[i].Query = normalizeQueryIndent(aa[i].Query)
	}

	*a = aa
	return nil
}

type Statements []struct {
	Total      float64 `db:"total"`
	MeanTime   float64 `db:"mean_time"`
	MinTime    float64 `db:"min_time"`
	MaxTime    float64 `db:"max_time"`
	StdDevTime float64 `db:"stddev_time"`
	Calls      int     `db:"calls"`
	HitPercent float64 `db:"hit_percent"`
	QueryID    int64   `db:"queryid"`
	Query      string  `db:"query"`
}

func (a *Statements) List(ctx context.Context, order string, asc bool, filter string) error {
	if order == "" {
		order = "total"
	}
	dir := "desc"
	if asc {
		dir = "asc"
	}

	var (
		args  []interface{}
		where string
	)
	if filter != "" {
		args = append(args, "%"+filter+"%")
		where = ` query like $1 `
	} else {
		where = ` calls >= 20 `
	}

	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`
		select
			(total_time / 1000 / 60) as total,
			mean_time,
			min_time,
			max_time,
			stddev_time,
			calls,
			coalesce(100.0 * shared_blks_hit / nullif(shared_blks_hit + shared_blks_read, 0), 0) as hit_percent,
			queryid,
			query
		from pg_stat_statements where
			userid = (select usesysid from pg_user where usename = CURRENT_USER) and
			query !~* '(^ *(copy|create|alter|explain) | (pg_stat_|pg_catalog)|^(COMMIT|BEGIN READ WRITE)$)' and
			%s
		order by %s %s, queryid asc
		limit 100
	`, where, order, dir), args...)
	if err != nil {
		return errors.Errorf("Statements.List: %w", err)
	}

	aa := *a
	for i := range aa {
		aa[i].Query = normalizeQueryIndent(aa[i].Query)
	}
	*a = aa
	return nil
}

type Tables []struct {
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

func (a *Tables) List(ctx context.Context) error {
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

			pg_table_size(  '"' || schemaname || '"."' || relname || '"'  ) / 1024/1024 as table_size,
			pg_indexes_size('"' || schemaname || '"."'  || relname || '"') / 1024/1024 as indexes_size

		from pg_stat_user_tables
		order by table_size desc
		-- order by n_dead_tup
		-- 	/(n_live_tup
		-- 	* current_setting('autovacuum_vacuum_scale_factor')::float8
		-- 	+ current_setting('autovacuum_vacuum_threshold')::float8)
		-- 	desc
	`)
	return errors.Wrap(err, "Tables.List")
}

type Indexes []struct {
	Table    string `db:"relname"`
	Size     int    `db:"size"`
	Index    string `db:"indexrelname"`
	Scan     int64  `db:"idx_scan"`
	TupRead  int64  `db:"idx_tup_read"`
	TupFetch int64  `db:"idx_tup_fetch"`
}

func (a *Indexes) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, `
		select
			relname,
			pg_relation_size('"' || schemaname || '"."' || indexrelname || '"') / 1024/1024 as size,
			indexrelname,
			idx_scan,
			idx_tup_read,
			idx_tup_fetch
		from pg_stat_user_indexes
		order by idx_scan desc
	`)
	return errors.Wrap(err, "Tables.List")
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

type Progress []struct {
	Table   string `db:"relname"`
	Command string `db:"command"`
	Phase   string `db:"phase"`
	Status  string `db:"status"`
}

func (a *Progress) List(ctx context.Context) error {
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

	return errors.Wrap(err, "Progress.List")
}

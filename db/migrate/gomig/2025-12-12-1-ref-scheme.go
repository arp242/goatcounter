package gomig

import (
	"context"
	"fmt"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/zdb"
)

func mergeIDs(ctx context.Context, dst, merge goatcounter.RefID) error {
	err := zdb.Exec(ctx, `
		insert into ref_counts (site_id, path_id, ref_id, hour, total)
			select site_id, path_id, ?, hour, total from ref_counts where ref_id=?
		`+goatcounter.Tables.RefCounts.OnConflict(ctx), dst, merge)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}
	err = zdb.Exec(ctx, `delete from ref_counts where ref_id=?`, merge)
	if err != nil {
		return fmt.Errorf("delete ref_counts: %w", err)
	}

	err = zdb.Exec(ctx, `update hits set ref_id=1 where ref_id=?`, merge)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	err = zdb.Exec(ctx, `delete from refs where ref_id=?`, merge)
	if err != nil {
		return fmt.Errorf("delete refs: %w", err)
	}
	return nil
}

func RefScheme(ctx context.Context) error {
	l := log.Module("migrate")

	// Create index so it's a bit faster; use "not not exists" so the index can
	// be created beforehand to speed up migration/decrease downtime.
	{
		l.Debug(ctx, "create index 1")
		err := zdb.Exec(ctx, `create index if not exists ref_scheme_tmp on refs(ref)`)
		if err != nil {
			return err
		}
	}
	{
		l.Debug(ctx, "create index 2")
		err := zdb.Exec(ctx, `create index if not exists ref_counts_tmp on ref_counts(ref_id)`)
		if err != nil {
			return err
		}
	}

	// Merge all refs where the ref is "" to id=1. Previously it would insert
	// that with different ref_schemes, and that was never intended.
	{
		l.Debug(ctx, "finding empty refs")
		var ids []goatcounter.RefID
		err := zdb.Select(ctx, &ids, `select ref_id from refs where ref='' and ref_id!=1`)
		if err != nil {
			return err
		}
		for _, id := range ids {
			l.Debugf(ctx, `merging ref="" id=%d ...`, id)
			err = mergeIDs(ctx, 1, id)
			if err != nil {
				return fmt.Errorf(`merging ref="" id=%d: %w`, id, err)
			}
		}
	}

	// ref_id=1 should have ref_scheme=o
	{
		l.Debug(ctx, "set ref_scheme='o'")
		err := zdb.Exec(ctx, `update refs set ref_scheme='o' where ref_id=1`)
		if err != nil {
			return fmt.Errorf("update ref_scheme on id=1: %w", err)
		}
	}

	// Merge all refs with ref_scheme=NULL to ref_scheme='o'; there are just a
	// few hundred of these on goatcounter.com so it's okay to just load that in
	// memory – I will assume that's the same for everyone.
	{
		var refs []struct {
			ID        goatcounter.RefID `db:"ref_id"`
			Ref       string            `db:"ref"`
			RefScheme *string           `db:"ref_scheme"`
		}
		l.Debug(ctx, "select * from refs where ref_scheme is null")
		err := zdb.Select(ctx, &refs, `select * from refs where ref_scheme is null`)
		if err != nil {
			return err
		}

		for i, r := range refs {
			// Find existing ref_scheme=o for this.
			var existing goatcounter.RefID
			err = zdb.Get(ctx, &existing, `select ref_id from refs where ref=? and ref_scheme='o'`, r.Ref)
			if err != nil && !zdb.ErrNoRows(err) {
				return err
			}

			if existing > 0 { // Merge with existing and delete
				l.Debugf(ctx, "%d/%d: merge %d to existing %d", i+1, len(refs), r.ID, existing)
				err := mergeIDs(ctx, existing, r.ID)
				if err != nil {
					return fmt.Errorf("merge to ref_scheme=o; id=%d: %w", r.ID, err)
				}
			} else { // No existing found: just change the ref_scheme
				l.Debugf(ctx, "%d/%d: update %d", i+1, len(refs), r.ID)
				err := zdb.Exec(ctx, `update refs set ref_scheme='o' where ref_id=?`, r.ID)
				if err != nil {
					return fmt.Errorf("update to ref_scheme=o; id=%d: %w", r.ID, err)
				}
			}
		}
	}

	// Update the table schema
	{
		l.Debug(ctx, "update table")
		if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
			for _, q := range []string{
				`create table refs2 (
					ref_id         integer        primary key autoincrement,
					ref            varchar        not null,
					ref_scheme     varchar        not null
				)`,
				`insert into refs2 select ref_id, ref, ref_scheme from refs`,
				`drop table refs`,
				`alter table refs2 rename to refs`,
				`create unique index "refs#ref#ref_scheme" on refs(lower(ref), ref_scheme)`,
			} {
				err := zdb.Exec(ctx, q)
				if err != nil {
					return fmt.Errorf("updating schema: %w", err)
				}
			}
		} else {
			err := zdb.Exec(ctx, `alter table refs alter column ref_scheme set not null`)
			if err != nil {
				return fmt.Errorf("updating schema: %w", err)
			}
		}
	}

	// Delete refs with newlines or carriage returns, or those that start or end
	// in a space. This is all junk.
	{
		chr := zdb.SQL("chr")
		if zdb.SQLDialect(ctx) == zdb.DialectSQLite {
			chr = "char"
		}
		var ids []goatcounter.RefID
		l.Debug(ctx, "finding malformed refs")
		err := zdb.Select(ctx, &ids, `
			select ref_id from refs where
				ref like '%' || :chr(10) || '%' or
				ref like '%' || :chr(13) || '%' or
				ref like ' %' or
				ref like '% '
		`, map[string]any{"chr": chr})
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			l.Debugf(ctx, "deleting %d malformed refs", len(ids))
			err := zdb.Exec(ctx, `delete from ref_counts where ref_id in (?)`, ids)
			if err != nil {
				return err
			}
			err = zdb.Exec(ctx, `delete from hits where ref_id in (?)`, ids)
			if err != nil {
				return err
			}
			err = zdb.Exec(ctx, `delete from refs where ref_id in (?)`, ids)
			if err != nil {
				return err
			}
		}
	}

	l.Debug(ctx, "dropping indexes")
	err := zdb.Exec(ctx, `drop index if exists ref_scheme_tmp`)
	if err != nil {
		return err
	}
	err = zdb.Exec(ctx, `drop index if exists ref_counts_tmp`)
	if err != nil {
		return err
	}

	return nil
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package gctest contains testing helpers.
package gctest

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zstring"
)

type tester interface {
	Helper()
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Logf(string, ...interface{})
}

var (
	dbname = "goatcounter_test_" + zcrypto.Secret64()
	db     *sqlx.DB
	tables []string
)

// DB starts a new database test.
func DB(t tester) (context.Context, func()) {
	t.Helper()
	ctx := context.Background()

	if db == nil {
		var err error
		if cfg.PgSQL {
			{
				out, err := exec.Command("createdb", dbname).CombinedOutput()
				if err != nil {
					t.Fatalf("%s → %s", err, out)
				}
			}

			db, err = sqlx.Connect("postgres", "dbname="+dbname+" sslmode=disable password=x")
		} else {
			db, err = sqlx.Connect("sqlite3", "file::memory:?cache=shared")
		}
		if err != nil {
			t.Fatalf("connect to DB: %s", err)
		}
		ctx = zdb.With(ctx, db)

		setupDB(t)

		tables, err = zdb.ListTables(ctx)
		if err != nil {
			t.Fatal(err)
		}

		exclude := []string{"iso_3166_1", "version"}
		tables = zstring.Filter(tables, func(t string) bool { return !zstring.Contains(exclude, t) })
	} else {
		ctx = zdb.With(ctx, db)

		q := `delete from %s`
		if cfg.PgSQL {
			// TODO: takes about 450ms, which is rather long. See if we can
			// speed this up.
			q = `truncate %s restart identity cascade`
		}
		for _, t := range tables {
			db.MustExec(fmt.Sprintf(q, t))
		}
		if !cfg.PgSQL {
			db.MustExec(`delete from sqlite_sequence`)
		}
	}

	goatcounter.Memstore.TestInit(db)
	ctx = initData(ctx, t)

	return ctx, func() {
		goatcounter.Memstore.Reset()

		// TODO: run after all tests are done.
		// out, err := exec.Command("dropdb", dbname).CombinedOutput()
		// if err != nil {
		// 	t.Logf("dropdb: %s → %s", err, out)
		// }
	}
}

func setupDB(t tester) {
	top, err := os.Getwd()
	if err != nil {
		t.Fatalf(fmt.Sprintf("cannot get cwd: %s", err))
	}
	for {
		if filepath.Base(top) == "goatcounter" {
			break
		}
		top = filepath.Dir(top)
		// Hit root path, I don't know how that will appear on Windows so check
		// the len(). Should never happen anyway.
		if len(top) < 5 {
			break
		}
	}
	schemapath := top + "/db/schema.sql"
	migratepath := top + "/db/migrate/sqlite"
	if cfg.PgSQL {
		schemapath = top + "/db/schema.pgsql"
		migratepath = top + "/db/migrate/pgsql"
	}

	s, err := ioutil.ReadFile(schemapath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema := string(s)
	_, err = db.ExecContext(context.Background(), schema)
	if err != nil {
		t.Fatalf("run schema %q: %v", schemapath, err)
	}

	migs, err := ioutil.ReadDir(migratepath)
	if err != nil {
		t.Fatalf("read migration directory: %s", err)
	}

	var migrations [][]string
	for _, m := range migs {
		if !strings.HasSuffix(m.Name(), ".sql") {
			continue
		}
		var ran bool
		db.Get(&ran, `select 1 from version where name=$1`, m.Name()[:len(m.Name())-4])
		if ran {
			continue
		}

		mp := fmt.Sprintf("%s/%s", migratepath, m.Name())
		mb, err := ioutil.ReadFile(mp)
		if err != nil {
			t.Fatalf("read migration: %s", err)
		}
		migrations = append(migrations, []string{mp, string(mb)})
	}

	for _, m := range migrations {
		_, err = db.ExecContext(context.Background(), m[1])
		if err != nil {
			t.Fatalf("run migration %q: %s", m[0], err)
		}
	}
	err = gomig.Run(db)
	if err != nil {
		t.Fatalf("gomig: %w", err)
	}
}

func initData(ctx context.Context, t tester) context.Context {
	{
		_, err := db.ExecContext(ctx, `insert into sites
			(code, plan, settings, created_at) values ('test', 'personal', '{}', $1)`,
			goatcounter.Now().Format(zdb.Date))
		if err != nil {
			t.Fatalf("create site: %s", err)
		}

		var site goatcounter.Site
		err = site.ByID(ctx, 1)
		if err != nil {
			t.Fatalf("get site: %s", err)
		}
		ctx = goatcounter.WithSite(ctx, &site)
	}

	{
		_, err := db.ExecContext(ctx, `insert into users
			(site, email, password, created_at) values (1, 'test@example.com', 'xx', $1)`,
			goatcounter.Now().Format(zdb.Date))
		if err != nil {
			t.Fatalf("create site: %s", err)
		}

		var user goatcounter.User
		err = user.BySite(ctx, 1)
		if err != nil {
			t.Fatalf("get user: %s", err)
		}
		ctx = goatcounter.WithUser(ctx, &user)
	}

	return ctx
}

// StoreHits is a convenient helper to store hits in the DB via Memstore and
// cron.UpdateStats().
func StoreHits(ctx context.Context, t *testing.T, wantFail bool, hits ...goatcounter.Hit) []goatcounter.Hit {
	t.Helper()

	for i := range hits {
		if hits[i].Session.IsZero() {
			hits[i].Session = goatcounter.TestSession
		}
		if hits[i].Site == 0 {
			hits[i].Site = 1
		}
	}

	goatcounter.Memstore.Append(hits...)
	hits, err := goatcounter.Memstore.Persist(ctx)
	if !wantFail && err != nil {
		t.Fatalf("gctest.StoreHits failed: %s", err)
	}
	if wantFail && err == nil {
		t.Fatal("gc.StoreHits: no error while wantError is true")
	}

	sites := make(map[int64]struct{})
	for _, h := range hits {
		sites[h.Site] = struct{}{}
	}

	for s := range sites {
		err = cron.UpdateStats(ctx, s, hits)
		if err != nil {
			t.Fatal(err)
		}
	}

	return hits
}

func Site(ctx context.Context, t *testing.T, site goatcounter.Site) (context.Context, goatcounter.Site) {
	if site.Code == "" {
		site.Code = zcrypto.Secret64()
		if len(site.Code) > 50 {
			site.Code = site.Code[:50]
		}
	}
	if site.Plan == "" {
		site.Plan = goatcounter.PlanPersonal
	}

	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx = goatcounter.WithSite(ctx, &site)

	var user goatcounter.User
	err = user.BySite(ctx, site.ID)
	if err != nil {
		user.Site = 1
		user.Email = "test@example.com"
		user.Password = []byte("coconuts")
		err = user.Insert(ctx)
	}
	if err != nil {
		t.Fatalf("get/create user: %s", err)
	}
	ctx = goatcounter.WithUser(ctx, &user)

	return ctx, site
}

func SwapNow(t *testing.T, date interface{}) func() {
	var (
		d   time.Time
		err error
	)
	switch dd := date.(type) {
	case string:
		d, err = time.Parse("2006-01-02 15:04:05", dd)
	case time.Time:
		d = dd
	case *time.Time:
		d = *dd
	default:
		t.Fatalf("unknown type: %T", date)
	}
	if err != nil {
		t.Fatal(err)
	}

	goatcounter.Now = func() time.Time { return d }
	return func() {
		goatcounter.Now = func() time.Time { return time.Now().UTC() }
	}
}

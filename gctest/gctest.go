// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// Package gctest contains testing helpers.
package gctest

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zhttp"
)

func init() {
	sql.Register("sqlite3_zdb", &sqlite3.SQLiteDriver{
		ConnectHook: goatcounter.SQLiteHook,
	})
}

var (
	schema     string
	migrations [][]string
)

type tester interface {
	Helper()
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Logf(string, ...interface{})
}

// DB starts a new database test.
func DB(t tester) (context.Context, func()) {
	t.Helper()

	clean := func() {}
	defer func() {
		r := recover()
		if r != nil {
			clean()
			panic(r)
		}
	}()

	dbname := "goatcounter_test_" + zhttp.Secret()[:25]

	if cfg.PgSQL {
		// TODO: avoid using shell commands if possible; it's quite slow!
		out, err := exec.Command("createdb", dbname).CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s → %s", err, out))
		}

		clean = func() {
			go func() {
				out, err := exec.Command("dropdb", dbname).CombinedOutput()
				if err != nil {
					t.Logf("dropdb: %s → %s", err, out)
				}
			}()
		}
	}

	var (
		db  *sqlx.DB
		err error
	)
	if cfg.PgSQL {
		db, err = sqlx.Connect("postgres", "dbname="+dbname+" sslmode=disable password=x")
	} else {
		db, err = sqlx.Connect("sqlite3_zdb", "file::memory:?cache=shared")
	}
	if err != nil {
		t.Fatalf("connect to DB: %s", err)
	}

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

	if schema == "" {
		s, err := ioutil.ReadFile(schemapath)
		if err != nil {
			t.Fatalf("read schema: %v", err)
		}
		schema = string(s)
		_, err = db.ExecContext(context.Background(), schema)
		if err != nil {
			t.Fatalf("run schema %q: %v", schemapath, err)
		}

		migs, err := ioutil.ReadDir(migratepath)
		if err != nil {
			t.Fatalf("read migration directory: %s", err)
		}

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
	} else {
		_, err = db.ExecContext(context.Background(), schema)
		if err != nil {
			t.Fatalf("create schema: %s", err)
		}
	}

	for _, m := range migrations {
		_, err = db.ExecContext(context.Background(), m[1])
		if err != nil {
			t.Fatalf("run migration %q: %s", m[0], err)
		}
	}

	now := `datetime()`
	if cfg.PgSQL {
		now = `now()`
	}

	_, err = db.ExecContext(context.Background(), fmt.Sprintf(
		`insert into sites (code, plan, settings, created_at) values
		('test', 'personal', '{}', %s);`, now))
	if err != nil {
		t.Fatalf("create site: %s", err)
	}

	ctx := zdb.With(context.Background(), db)

	var site goatcounter.Site
	err = site.ByID(ctx, 1)
	if err != nil {
		t.Fatalf("get site: %s", err)
	}
	ctx = goatcounter.WithSite(ctx, &site)
	ctx = goatcounter.WithUser(ctx, &goatcounter.User{ID: 1, Site: 1})

	return ctx, func() {
		db.Close()
		goatcounter.Salts.Clear()
		clean()
	}
}

// StoreHits is a convenient helper to store hits in the DB via Memstore and
// cron.UpdateStats().
func StoreHits(ctx context.Context, t *testing.T, hits ...goatcounter.Hit) []goatcounter.Hit {
	t.Helper()

	one := int64(1)
	for i := range hits {
		if hits[i].Session == nil || *hits[i].Session == 0 {
			hits[i].Session = &one
		}
		if hits[i].Site == 0 {
			hits[i].Site = 1
		}
	}

	goatcounter.Memstore.Append(hits...)
	hits, err := goatcounter.Memstore.Persist(ctx)
	if err != nil {
		t.Fatal(err)
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
		site.Code = zhttp.Secret()
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

	u := goatcounter.User{
		Site:     site.ID,
		Email:    "martin@arp242.net",
		Password: []byte("goatcounter"),
	}
	err = u.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return ctx, site
}

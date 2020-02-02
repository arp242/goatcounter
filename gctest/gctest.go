// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

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

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
)

var (
	schema     string
	migrations []string
)

// DB starts a new database test.
func DB(t *testing.T) (context.Context, func()) {
	t.Helper()

	dbname := "goatcounter_test_" + zhttp.Secret()

	if cfg.PgSQL {
		out, err := exec.Command("createdb", dbname).CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s → %s", err, out))
		}
	}

	var (
		db  *sqlx.DB
		err error
	)
	if cfg.PgSQL {
		db, err = sqlx.Connect("postgres", "dbname="+dbname+" sslmode=disable password=x")
	} else {
		db, err = sqlx.Connect("sqlite3", ":memory:")
	}
	if err != nil {
		t.Fatal(err)
	}

	top, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("cannot get cwd: %s", err))
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
			t.Fatal(err)
		}
		schema = string(s)
		_, err = db.ExecContext(context.Background(), schema)
		if err != nil {
			t.Fatal(err)
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

			mb, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", migratepath, m.Name()))
			if err != nil {
				t.Fatalf("read migration: %s", err)
			}
			migrations = append(migrations, string(mb))
		}
	} else {
		_, err = db.ExecContext(context.Background(), schema)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, m := range migrations {
		_, err = db.ExecContext(context.Background(), m)
		if err != nil {
			t.Fatal(err)
		}
	}

	now := `datetime()`
	if cfg.PgSQL {
		now = `now()`
	}

	_, err = db.ExecContext(context.Background(), fmt.Sprintf(
		`insert into sites (code, name, plan, settings, created_at) values
		('test', 'example.com', 'personal', '{}', %s);`, now))
	if err != nil {
		t.Fatal(err)
	}

	ctx := zdb.With(context.Background(), db)
	ctx = context.WithValue(ctx, ctxkey.Site, &goatcounter.Site{ID: 1})
	ctx = context.WithValue(ctx, ctxkey.User, &goatcounter.User{ID: 1, Site: 1})

	return ctx, func() {
		db.Close()
		if cfg.PgSQL {
			out, err := exec.Command("dropdb", dbname).CombinedOutput()
			if err != nil {
				panic(fmt.Sprintf("%s → %s", err, out))
			}
		}
	}
}

// StoreHits is a convenient helper to store hits in the DB via Memstore and
// cron.UpdateStats().
func StoreHits(ctx context.Context, t *testing.T, hits ...goatcounter.Hit) []goatcounter.Hit {
	t.Helper()

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
	if site.Name == "" {
		site.Name = "name"
	}
	if site.Plan == "" {
		site.Plan = goatcounter.PlanPersonal
	}

	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return context.WithValue(ctx, ctxkey.Site, &site), site
}

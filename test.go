// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
)

var (
	schema     string
	migrations []string
)

// StartTest a new database test.
func StartTest(t *testing.T) (context.Context, func()) {
	t.Helper()

	var (
		db  *sqlx.DB
		err error
	)

	if cfg.PgSQL {
		db, err = sqlx.Connect("postgres", "dbname=goatcounter_test sslmode=disable password=x")
	} else {
		db, err = sqlx.Connect("sqlite3", ":memory:")
	}
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PgSQL {
		cleanpg(t, db)
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
		_, err = db.Exec(schema)
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
		_, err = db.Exec(schema)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, m := range migrations {
		_, err = db.Exec(m)
		if err != nil {
			t.Fatal(err)
		}
	}

	now := `datetime()`
	if cfg.PgSQL {
		now = `now()`
	}

	_, err = db.Exec(fmt.Sprintf(`insert into sites (code, name, plan, settings, created_at) values
		('test', 'example.com', 'personal', '{}', %s);`, now))
	if err != nil {
		t.Fatal(err)
	}

	ctx := zdb.With(context.Background(), db)
	ctx = context.WithValue(ctx, ctxkey.Site, &Site{ID: 1})
	ctx = context.WithValue(ctx, ctxkey.User, &User{ID: 1, Site: 1})

	return ctx, func() {
		if cfg.PgSQL {
			cleanpg(t, db)
		}
		db.Close()
	}
}

func cleanpg(t *testing.T, db *sqlx.DB) {
	_, err := db.Exec("drop schema public cascade;")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("create schema public;")
	if err != nil {
		t.Fatal(err)
	}
}

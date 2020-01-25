// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// +build testpg

package goatcounter

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
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
)

var (
	schema     string
	migrations []string
)

func init() {
	// Doing this on every test run doubles the running time.
	exec.Command("dropdb", "goatcounter_test").CombinedOutput()
	out, err := exec.Command("createdb", "goatcounter_test").CombinedOutput()
	if err != nil {
		panic(string(out))
	}
}

// StartTest a new database test.
func StartTest(t *testing.T) (context.Context, func()) {
	t.Helper()

	cfg.PgSQL = true

	db, err := sqlx.Connect("postgres", "dbname=goatcounter_test sslmode=disable password=x")
	if err != nil {
		t.Fatal(err)
	}
	cleanpg(t, db)

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

	if schema == "" {
		s, err := ioutil.ReadFile(top + "/db/schema.pgsql")
		if err != nil {
			t.Fatal(err)
		}
		schema = string(s)
		_, err = db.Exec(schema)
		if err != nil {
			t.Fatal(err)
		}

		migs, err := ioutil.ReadDir(top + "/db/migrate/pgsql")
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

			mb, err := ioutil.ReadFile(fmt.Sprintf("%s/db/migrate/pgsql/%s", top, m.Name()))
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

	_, err = db.Exec(`insert into sites (code, name, plan, settings, created_at) values
		('test', 'example.com', 'personal', '{}', now());`)
	if err != nil {
		t.Fatal(err)
	}

	ctx := zdb.With(context.Background(), db)
	ctx = context.WithValue(ctx, ctxkey.Site, &Site{ID: 1})
	ctx = context.WithValue(ctx, ctxkey.User, &User{ID: 1, Site: 1})

	return ctx, func() {
		cleanpg(t, db)
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

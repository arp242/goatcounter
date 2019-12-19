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

	db, err := sqlx.Connect("sqlite3", ":memory:")
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

	if schema == "" {
		schema, err := ioutil.ReadFile(top + "/db/schema.sql")
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(string(schema))
		if err != nil {
			t.Fatal(err)
		}

		migs, err := ioutil.ReadDir(top + "/db/migrate/sqlite")
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

			mb, err := ioutil.ReadFile(fmt.Sprintf("%s/db/migrate/sqlite/%s", top, m.Name()))
			if err != nil {
				t.Fatalf("read migration: %s", err)
			}
			migrations = append(migrations, string(mb))
		}
	}

	for _, m := range migrations {
		_, err = db.Exec(m)
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec(`insert into sites (code, name, plan, settings, created_at) values
		('test', 'example.com', 'personal', '{}', datetime());`)
	if err != nil {
		t.Fatal(err)
	}

	ctx := zdb.With(context.Background(), db)
	ctx = context.WithValue(ctx, ctxkey.Site, &Site{ID: 1})
	ctx = context.WithValue(ctx, ctxkey.User, &User{ID: 1, Site: 1})

	return ctx, func() { db.Close() }
}

package goatcounter

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
)

var schema string

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
		schemaB, err := ioutil.ReadFile(top + "/db/schema.sql")
		if err != nil {
			t.Fatal(err)
		}
		schema = string(schemaB)
	}

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`insert into sites (code, name, plan, settings, created_at) values
		('test', 'example.com', 'p', '{}', datetime());`)
	if err != nil {
		t.Fatal(err)
	}

	ctx := zdb.With(context.Background(), db)
	ctx = context.WithValue(ctx, ctxkey.Site, &Site{ID: 1})
	ctx = context.WithValue(ctx, ctxkey.User, &User{ID: 1, Site: 1})

	return ctx, func() { db.Close() }
}

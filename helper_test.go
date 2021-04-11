// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"testing"
	"testing/fstest"

	"zgo.at/zdb"
)

func TestEmbed(t *testing.T) {
	err := fstest.TestFS(DB, "db/schema.gotxt", "db/migrate/2020-08-28-1-paths-tables-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}

	err = fstest.TestFS(DB, "db/goatcounter.sqlite3", "db/migrate/gomig/gomig.go")
	if err == nil {
		t.Fatal("db/goatcounter.sqlite3 in embeded files")
	}
}

func TestSQLiteJSON(t *testing.T) {
	ctx := zdb.StartTest(t)
	if zdb.Driver(ctx) != zdb.DriverSQLite {
		return
	}

	var out string
	err := zdb.Get(ctx, &out, `select json('["a"  ,  "b"]')`)
	if err != nil {
		t.Fatal(err)
	}

	want := `["a","b"]`
	if out != want {
		t.Errorf("\ngot:  %q\nwant: %q", out, want)
	}
}

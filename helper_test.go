// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"context"
	"testing"
	"testing/fstest"

	"zgo.at/zdb"
)

func TestEmbed(t *testing.T) {
	err := fstest.TestFS(DB, "db/schema.gotxt", "db/migrate/2022-10-17-1-campaigns.gotxt")
	if err != nil {
		t.Fatal(err)
	}

	err = fstest.TestFS(DB, "db/goatcounter.sqlite3", "db/migrate/gomig/gomig.go")
	if err == nil {
		t.Fatal("db/goatcounter.sqlite3 in embeded files")
	}
}

func TestSQLiteJSON(t *testing.T) {
	zdb.RunTest(t, func(t *testing.T, ctx context.Context) {
		if zdb.SQLDialect(ctx) != zdb.DialectSQLite {
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
	})
}

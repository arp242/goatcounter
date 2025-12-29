//go:build cgo

package goatcounter_test

import (
	"testing"

	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
)

func TestSQLiteJSON(t *testing.T) {
	ctx := gctest.DB(t)
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
}

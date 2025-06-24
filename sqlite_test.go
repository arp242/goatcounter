//go:build cgo

package goatcounter

import (
	"context"
	"testing"

	"zgo.at/zdb"
)

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

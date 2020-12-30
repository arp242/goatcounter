// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// +build !testpg

package goatcounter

import (
	"testing"

	"zgo.at/zdb"
)

func TestSQLiteJSON(t *testing.T) {
	ctx, clean := zdb.StartTest(t)
	defer clean()

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

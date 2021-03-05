// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"testing"
)

func TestMigrate(t *testing.T) {
	exit, _, out, _, dbc, clean := startTest(t)
	defer clean()

	runCmd(t, exit, "migrate", "-db="+dbc, "show")
	wantExit(t, exit, out, 0)
	want := "No pending migrations\n"
	if out.String() != want {
		t.Error()
	}
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"strings"
	"testing"
)

func TestMigrate(t *testing.T) {
	ctx, dbc, clean := tmpdb(t)
	defer clean()

	out, code := run(t, "", []string{"migrate",
		"-db", dbc})
	if code != 0 {
		t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
	}
	_ = ctx
}

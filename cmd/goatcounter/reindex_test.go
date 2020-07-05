// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"strings"
	"testing"
)

func TestReindex(t *testing.T) {
	ctx, dbc, clean := tmpdb(t)
	defer clean()

	out, code := run(t, "", []string{"reindex", "-db", dbc})
	if code != 0 {
		t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
	}
	_ = ctx
}

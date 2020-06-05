// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestServe(t *testing.T) {
	// I don't know why, but this doesn't work in Windows; I think it may be
	// related to permission issues for binding to a port or some such?
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	ctx, dbc, clean := tmpdb(t)
	defer clean()

	out, code := run(t, "serving", []string{"serve",
		"-listen", "localhost:31874",
		"-db", dbc})
	if code != 0 {
		t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
	}
	_ = ctx
}

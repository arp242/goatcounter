// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestSaas(t *testing.T) {
	// I don't know why, but this doesn't work in Windows; I think it may be
	// related to permission issues for binding to a port or some such?
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	ctx, dbc, clean := tmpdb(t)
	defer clean()

	out, code := run(t, "serving", []string{"saas",
		"-domain", "goatcounter.com,a.a",
		"-listen", "localhost:31874",
		"-tls", "none",
		"-stripe", "sk_test_x:pk_test_x:whsec_x",
		"-db", dbc})
	if code != 0 {
		t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
	}
	_ = ctx
}

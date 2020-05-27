// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"strings"
	"testing"
)

func TestSaas(t *testing.T) {
	ctx, dbc, clean := tmpdb(t)
	defer clean()

	out, code := run(t, "serving", []string{"saas",
		"-domain", "goatcounter.com,a.a",
		"-listen", "localhost:31874",
		"-stripe", "sk_test_x:pk_test_x:whsec_x",
		"-db", dbc})
	if code != 0 {
		t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
	}
	_ = ctx
}

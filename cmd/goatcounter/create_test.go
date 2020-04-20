// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"strings"
	"testing"

	"zgo.at/goatcounter"
)

func TestCreate(t *testing.T) {
	ctx, dbc, clean := tmpdb(t)
	defer clean()

	out, code := run(t, "", []string{"create",
		"-email", "foo@foo.foo",
		"-domain", "stats.stats",
		"-password", "password",
		"-db", dbc})
	if code != 0 {
		t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
	}

	var s goatcounter.Site
	err := s.ByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	var u goatcounter.User
	err = u.BySite(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
}

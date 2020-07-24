// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"testing"

	"zgo.at/goatcounter"
)

func TestCreate(t *testing.T) {
	ctx, dbc, clean := tmpdb(t)
	defer clean()

	run(t, 0, []string{"create",
		"-email", "foo@foo.foo",
		"-domain", "stats.stats",
		"-password", "password",
		"-db", dbc})

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

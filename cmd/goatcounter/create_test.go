// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"testing"

	"zgo.at/goatcounter"
)

func TestCreate(t *testing.T) {
	exit, _, out, ctx, dbc, clean := startTest(t)
	defer clean()

	{
		runCmd(t, exit, "create",
			"-db="+dbc,
			"-email=foo@foo.foo",
			"-domain=stats.stats",
			"-password=password")
		wantExit(t, exit, out, 0)

		var s goatcounter.Site
		err := s.ByID(ctx, 2)
		if err != nil {
			t.Fatal(err)
		}
		var u goatcounter.User
		err = u.BySite(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
	}

	{
		runCmd(t, exit, "create",
			"-db="+dbc,
			"-parent=1",
			"-domain=stats2.stats",
			"-password=password")
		wantExit(t, exit, out, 0)

		var s goatcounter.Site
		err := s.ByID(ctx, 3)
		if err != nil {
			t.Fatal(err)
		}
		if *s.Parent != 1 {
			t.Fatalf("s.Parent = %d", *s.Parent)
		}
		var u goatcounter.User
		err = u.BySite(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
}

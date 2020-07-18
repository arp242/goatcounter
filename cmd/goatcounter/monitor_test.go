// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"strings"
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
)

func TestMonitor(t *testing.T) {
	ctx, dbc, clean := tmpdb(t)
	defer clean()

	t.Run("no pageviews", func(t *testing.T) {
		out, code := run(t, "", []string{"monitor",
			"-db", dbc,
			"-once",
			"-debug", "all"})
		if code != 1 {
			t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
		}
	})

	t.Run("with pageviews", func(t *testing.T) {
		ctx, site := gctest.Site(ctx, t, goatcounter.Site{})
		gctest.StoreHits(ctx, t, goatcounter.Hit{Path: "/", Site: site.ID})

		out, code := run(t, "", []string{"monitor",
			"-db", dbc,
			"-once",
			"-debug", "all"})
		if code != 0 {
			t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
		}
	})
}

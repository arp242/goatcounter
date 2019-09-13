// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package cron

import (
	"fmt"
	"testing"
	"time"

	"github.com/teamwork/utils/jsonutil"
	"zgo.at/goatcounter"
)

func TestHitStats(t *testing.T) {
	ctx, clean := goatcounter.StartTest(t)
	defer clean()

	site := goatcounter.MustGetSite(ctx)
	now := time.Date(2019, 8, 31, 14, 42, 0, 0, time.UTC)

	// Insert some hits.
	hits := []goatcounter.Hit{
		{Path: "/asd", CreatedAt: now},
		{Path: "/asd", CreatedAt: now},
		{Path: "/zxc", CreatedAt: now},
	}
	for _, h := range hits {
		h.Site = site.ID
		err := h.Insert(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := updateStats(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var stats goatcounter.HitStats
	total, display, more, err := stats.List(ctx, now, now, nil)
	if err != nil {
		t.Fatal(err)
	}

	if total != 3 || display != 3 || more {
		t.Errorf("wrong return\nwant: 3, 3, false\ngot:  %v, %v, %v", total, display, more)
	}

	if len(stats) != 2 {
		fmt.Printf("len(stats) is not 2: %d", len(stats))
	}

	want0 := `{"Count":2,"Max":10,"Path":"/asd","RefScheme":null,"Stats":[{"Day":"2019-08-31","Days":[[0,0],[1,0],[2,0],[3,0],[4,0],[5,0],[6,0],[7,0],[8,0],[9,0],[10,0],[11,0],[12,0],[13,0],[14,2],[15,0],[16,0],[17,0],[18,0],[19,0],[20,0],[21,0],[22,0],[23,0]]}]}`
	got0 := string(jsonutil.MustMarshal(stats[0]))
	if got0 != want0 {
		t.Errorf("first wrong\ngot:  %s\nwant: %s", got0, want0)
	}

	want1 := `{"Count":1,"Max":10,"Path":"/zxc","RefScheme":null,"Stats":[{"Day":"2019-08-31","Days":[[0,0],[1,0],[2,0],[3,0],[4,0],[5,0],[6,0],[7,0],[8,0],[9,0],[10,0],[11,0],[12,0],[13,0],[14,1],[15,0],[16,0],[17,0],[18,0],[19,0],[20,0],[21,0],[22,0],[23,0]]}]}`
	got1 := string(jsonutil.MustMarshal(stats[1]))
	if got1 != want1 {
		t.Errorf("second wrong\ngot:  %s\nwant: %s", got1, want1)
	}
}

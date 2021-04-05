// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"testing"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/ztime"
)

func TestReindex(t *testing.T) {
	ztime.SetNow(t, "2020-06-18 12:00:00")
	exit, _, out, ctx, dbc := startTest(t)

	gctest.StoreHits(ctx, t, false, goatcounter.Hit{})

	tables := []string{"hit_stats", "system_stats", "browser_stats",
		"location_stats", "size_stats", "hit_counts", "ref_counts"}

	for _, tbl := range tables {
		err := zdb.Exec(ctx, `delete from `+tbl)
		if err != nil {
			t.Fatal(err)
		}
	}

	runCmd(t, exit, "reindex", "-db="+dbc)
	wantExit(t, exit, out, 0)

	var got string
	for _, tbl := range tables {
		got += zdb.DumpString(ctx, `select * from `+tbl)
	}

	want := `
		site_id  path_id  day                  stats                                              stats_unique
		1        1        2020-06-18 00:00:00  [0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0]  [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
		site_id  path_id  system_id  day                  count  count_unique
		1        1        1          2020-06-18 00:00:00  1      0
		site_id  path_id  browser_id  day                  count  count_unique
		1        1        1           2020-06-18 00:00:00  1      0
		site_id  path_id  day                  location  count  count_unique
		1        1        2020-06-18 00:00:00            1      0
		site_id  path_id  day                  width  count  count_unique
		1        1        2020-06-18 00:00:00  0      1      0
		site_id  path_id  hour                 total  total_unique
		1        1        2020-06-18 12:00:00  1      0
		site_id  path_id  ref  ref_scheme  hour                 total  total_unique
		1        1             NULL        2020-06-18 12:00:00  1      0`

	if d := zdb.Diff(got, want); d != "" {
		t.Error(d)
	}
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/ztest"
)

func TestHitStatsList(t *testing.T) {
	start := time.Date(2019, 8, 10, 14, 42, 0, 0, time.UTC)
	end := time.Date(2019, 8, 17, 14, 42, 0, 0, time.UTC)

	tests := []struct {
		in         []goatcounter.Hit
		inFilter   string
		inExclude  []string
		wantReturn string
		wantStats  goatcounter.HitStats
	}{
		{
			in: []goatcounter.Hit{
				{CreatedAt: start, Path: "/asd"},
				{CreatedAt: start.Add(40 * time.Hour), Path: "/asd/"},
				{CreatedAt: start.Add(100 * time.Hour), Path: "/zxc"},
			},
			wantReturn: "3 3 false <nil>",
			wantStats: goatcounter.HitStats{
				goatcounter.HitStat{Count: 2, Max: 10, Path: "/asd", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-11", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-12", Days: []int{0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-13", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-14", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-15", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-16", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-17", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
				}},
				goatcounter.HitStat{Count: 1, Max: 10, Path: "/zxc", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-11", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-12", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-13", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-14", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0}},
					{Day: "2019-08-15", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-16", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-17", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
				}},
			},
		},
		{
			in: []goatcounter.Hit{
				{CreatedAt: start, Path: "/asd"},
				{CreatedAt: start, Path: "/zxc"},
			},
			inFilter:   "x",
			wantReturn: "1 1 false <nil>",
			wantStats: goatcounter.HitStats{
				goatcounter.HitStat{Count: 1, Max: 10, Path: "/zxc", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-11", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-12", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-13", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-14", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-15", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-16", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
					{Day: "2019-08-17", Days: []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
				}},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx, clean := goatcounter.StartTest(t)
			defer clean()

			site := goatcounter.MustGetSite(ctx)
			for j := range tt.in {
				if tt.in[j].Site == 0 {
					tt.in[j].Site = site.ID
				}
			}

			goatcounter.Memstore.Append(tt.in...)
			db := zdb.MustGet(ctx).(*sqlx.DB)
			cron.Run(db)

			var stats goatcounter.HitStats
			total, totalDisplay, more, err := stats.List(ctx, start, end, tt.inFilter, tt.inExclude)

			got := fmt.Sprintf("%d %d %t %v", total, totalDisplay, more, err)
			if got != tt.wantReturn {
				t.Fatalf("wrong return\nout:  %s\nwant: %s\n", got, tt.wantReturn)
			}

			if d := ztest.Diff(stats, tt.wantStats); d != "" {
				t.Fatal(d)
			}
		})
	}
}

func TestHitDefaults(t *testing.T) {
	a := "arp242.net"
	set := ztest.SP("_")

	tests := []struct {
		in           string
		wantRef      string
		wantParams   *string
		wantOriginal *string
		wantScheme   string
	}{
		// Split out query parameters.
		{"https://arp242.net", a, nil, nil, "h"},
		{"https://arp242.net?a=b", a, ztest.SP("a=b"), nil, "h"},
		{"https://arp242.net?a=b&c=d", a, ztest.SP("a=b&c=d"), nil, "h"},

		// Clean up query parameters.
		{"https://t.co/asd?amp=1", "t.co/asd", nil, nil, "h"},
		{"https://arp242.net?utm_source=asd", a, nil, set, "h"},
		{"https://arp242.net?utm_source=asd&a=b", a, ztest.SP("a=b"), set, "h"},

		// Groups
		{"https://mail.google.com?a=b&c=d", "Email", nil, set, "g"},
		{"android-app://com.laurencedawson.reddit_sync.pro", "www.reddit.com", nil, set, "g"},

		// Host aliases.
		{"https://en.m.wikipedia.org/wiki/Foo", "en.wikipedia.org/wiki/Foo", nil, set, "h"},
		{"https://en.m.wikipedia.org/wiki/Foo?a=b", "en.wikipedia.org/wiki/Foo", ztest.SP("a=b"), set, "h"},

		// Reddit Cleaning.
		{"https://www.reddit.com/r/programming/top", "www.reddit.com/r/programming", nil, set, "h"},
		{"https://np.reddit.com/r/programming/.compact", "www.reddit.com/r/programming", nil, set, "h"},

		{"android-app://com.example.android", "com.example.android", nil, nil, "o"},
	}

	ctx := context.WithValue(context.Background(), ctxkey.Site, &goatcounter.Site{ID: 1})

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			h := goatcounter.Hit{Ref: tt.in}
			h.RefURL, _ = url.Parse(tt.in)
			h.Defaults(ctx)

			if tt.wantOriginal != nil && *tt.wantOriginal == "_" {
				tt.wantOriginal = &tt.in
			}

			if h.Ref != tt.wantRef {
				t.Fatalf("wrong Ref\nout:  %#v\nwant: %#v\n",
					h.Ref, tt.wantRef)
			}
			if !CmpString(h.RefParams, tt.wantParams) {
				t.Fatalf("wrong RefParams\nout:  %#v\nwant: %#v\n",
					PSP(h.RefParams), PSP(tt.wantParams))
			}
			if !CmpString(h.RefOriginal, tt.wantOriginal) {
				t.Fatalf("wrong RefOriginal\nout:  %#v\nwant: %#v\n",
					PSP(h.RefOriginal), PSP(tt.wantOriginal))
			}
			if *h.RefScheme != tt.wantScheme {
				t.Fatalf("wrong RefScheme\nout:  %#v\nwant: %#v\n",
					PSP(h.RefScheme), tt.wantScheme)
			}
		})
	}
}

func CmpString(out, want *string) bool {
	if out == nil && want == nil {
		return true
	}
	if out == nil && want != nil {
		return false
	}
	if want == nil && out != nil {
		return false
	}
	if *out != *want {
		return false
	}
	return true
}

func PSP(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

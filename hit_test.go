// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/ztest"
)

func dayStat(days map[int]int) []int {
	s := make([]int, 24)
	for k, v := range days {
		s[k] = v
	}
	return s
}

func TestHitStatsList(t *testing.T) {
	start := time.Date(2019, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2019, 8, 17, 23, 59, 59, 0, time.UTC)
	hit := start.Add(1 * time.Second)

	tests := []struct {
		in         []goatcounter.Hit
		inFilter   string
		inExclude  []string
		wantReturn string
		wantStats  goatcounter.HitStats
	}{
		{
			in: []goatcounter.Hit{
				{CreatedAt: hit, Path: "/asd"},
				{CreatedAt: hit.Add(40 * time.Hour), Path: "/asd/"},
				{CreatedAt: hit.Add(100 * time.Hour), Path: "/zxc"},
			},
			wantReturn: "3 3 false <nil>",
			wantStats: goatcounter.HitStats{
				goatcounter.HitStat{Count: 2, Path: "/asd", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(map[int]int{6: 1})},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
				goatcounter.HitStat{Count: 1, Path: "/zxc", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(nil)},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(map[int]int{18: 1})},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
		{
			in: []goatcounter.Hit{
				{CreatedAt: hit, Path: "/asd"},
				{CreatedAt: hit, Path: "/zxc"},
			},
			inFilter:   "x",
			wantReturn: "1 1 false <nil>",
			wantStats: goatcounter.HitStats{
				goatcounter.HitStat{Count: 1, Path: "/zxc", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
		{
			in: []goatcounter.Hit{
				{CreatedAt: hit, Path: "/a"},
				{CreatedAt: hit, Path: "/aa"},
				{CreatedAt: hit, Path: "/aaa"},
				{CreatedAt: hit, Path: "/aaaa"},
			},
			inFilter:   "a",
			wantReturn: "4 2 true <nil>",
			wantStats: goatcounter.HitStats{
				goatcounter.HitStat{Count: 1, Path: "/aaaa", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
				goatcounter.HitStat{Count: 1, Path: "/aaa", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
		{
			in: []goatcounter.Hit{
				{CreatedAt: hit, Path: "/a"},
				{CreatedAt: hit, Path: "/aa"},
				{CreatedAt: hit, Path: "/aaa"},
				{CreatedAt: hit, Path: "/aaaa"},
			},
			inFilter:   "a",
			inExclude:  []string{"/aaaa", "/aaa"},
			wantReturn: "4 2 false <nil>",
			wantStats: goatcounter.HitStats{
				goatcounter.HitStat{Count: 1, Path: "/aa", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
				goatcounter.HitStat{Count: 1, Path: "/a", RefScheme: nil, Stats: []goatcounter.Stat{
					{Day: "2019-08-10", Hourly: dayStat(map[int]int{14: 1})},
					{Day: "2019-08-11", Hourly: dayStat(nil)},
					{Day: "2019-08-12", Hourly: dayStat(nil)},
					{Day: "2019-08-13", Hourly: dayStat(nil)},
					{Day: "2019-08-14", Hourly: dayStat(nil)},
					{Day: "2019-08-15", Hourly: dayStat(nil)},
					{Day: "2019-08-16", Hourly: dayStat(nil)},
					{Day: "2019-08-17", Hourly: dayStat(nil)},
				}},
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx, clean := gctest.DB(t)
			defer clean()

			site := goatcounter.MustGetSite(ctx)
			for j := range tt.in {
				if tt.in[j].Site == 0 {
					tt.in[j].Site = site.ID
				}
			}
			site.Settings.Limits.Page = 2

			gctest.StoreHits(ctx, t, tt.in...)

			var stats goatcounter.HitStats
			total, totalUnique, totalDisplay, uniqueDisplay, more, err := stats.List(
				ctx, start, end, tt.inFilter, tt.inExclude, false)
			_, _ = totalUnique, uniqueDisplay // TODO

			got := fmt.Sprintf("%d %d %t %v", total, totalDisplay, more, err)
			if got != tt.wantReturn {
				t.Errorf("wrong return\nout:  %s\nwant: %s\n", got, tt.wantReturn)
			}

			out := strings.ReplaceAll(", ", ",\n", fmt.Sprintf("%+v", stats))
			want := strings.ReplaceAll(", ", ",\n", fmt.Sprintf("%+v", tt.wantStats))
			if d := ztest.Diff(out, want); d != "" {
				t.Fatal(d)
			}
		})
	}
}

func TestHitDefaultsRef(t *testing.T) {
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

	ctx := goatcounter.WithSite(context.Background(), &goatcounter.Site{ID: 1})

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
			if *h.RefScheme != tt.wantScheme {
				t.Fatalf("wrong RefScheme\nout:  %#v\nwant: %#v\n",
					PSP(h.RefScheme), tt.wantScheme)
			}
		})
	}
}

func TestHitDefaultsPath(t *testing.T) {
	tests := []struct {
		in       string
		wantPath string
	}{
		{"/page", "/page"},
		{"//page/", "/page"},
		{"//", "/"},
		{"", "/"},

		{"/page?q=a", "/page?q=a"},
		{"/page?fbclid=foo", "/page"},
		{"/page?fbclid=foo&a=b", "/page?a=b"},
		{"/page?", "/page"},
		{"/page?", "/page"},

		{
			"/storage/emulated/0/Android/data/jonas.tool.saveForOffline/files/Curl_to_shell_isn_t_so_bad2019-11-09-11-07-58/curl-to-sh.html",
			"/curl-to-sh.html",
		},

		{"/web/20200104233523/https://www.arp242.net/tmux.html", "/tmux.html"},
		{"/web/20190820072242/https://arp242.net", "/"},
		{"/web/20190820072242/https://arp242.net?a=b", "/?a=b"},
		{"/web/20190820072242/https://arp242.net?a=b&c=d", "/?a=b&c=d"},
		{"/web/20200104233523/https://www.arp242.net/many/more/slashes", "/many/more/slashes"},
		{"/web/assets/images/social-github.svg", "/web/assets/images/social-github.svg"},
	}

	ctx := goatcounter.WithSite(context.Background(), &goatcounter.Site{ID: 1})

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			h := goatcounter.Hit{Path: tt.in}
			h.Defaults(ctx)

			if h.Path != tt.wantPath {
				t.Fatalf("wrong Path\nout:  %#v\nwant: %#v\n",
					h.Path, tt.wantPath)
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

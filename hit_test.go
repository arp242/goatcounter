// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"net/url"
	"testing"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/ztype"
)

func dayStat(days map[int]int) []int {
	s := make([]int, 24)
	for k, v := range days {
		s[k] = v
	}
	return s
}

func TestHitDefaultsRef(t *testing.T) {
	a := "arp242.net"
	set := ztype.Ptr("_")

	tests := []struct {
		in           string
		wantRef      string
		wantParams   *string
		wantOriginal *string
		wantScheme   string
	}{
		{"", "", nil, nil, ""},

		{"xx:", "", nil, nil, "o"}, // Empty as "xx:" is parsed as the scheme.

		// Split out query parameters.
		{"https://arp242.net", a, nil, nil, "h"},
		{"https://arp242.net?a=b", a, ztype.Ptr("a=b"), nil, "h"},
		{"https://arp242.net?a=b&c=d", a, ztype.Ptr("a=b&c=d"), nil, "h"},

		// Clean up query parameters.
		{"https://t.co/asd", "twitter.com/search?q=https%3A%2F%2Ft.co%2Fasd", nil, nil, "h"},
		{"https://t.co/asd?amp=1", "twitter.com/search?q=https%3A%2F%2Ft.co%2Fasd", nil, nil, "h"},
		{"https://arp242.net?utm_source=asd", a, nil, set, "h"},
		{"https://arp242.net?utm_source=asd&a=b", a, ztype.Ptr("a=b"), set, "h"},

		// Groups
		{"https://mail.google.com?a=b&c=d", "Email", nil, set, "g"},
		{"android-app://com.laurencedawson.reddit_sync.pro", "www.reddit.com", nil, set, "g"},

		// Host aliases.
		{"https://en.m.wikipedia.org/wiki/Foo", "en.wikipedia.org/wiki/Foo", nil, set, "h"},
		{"https://en.m.wikipedia.org/wiki/Foo?a=b", "en.wikipedia.org/wiki/Foo", ztype.Ptr("a=b"), set, "h"},

		// Reddit Cleaning.
		{"https://www.reddit.com/r/programming/top", "www.reddit.com/r/programming", nil, set, "h"},
		{"https://np.reddit.com/r/programming/.compact", "www.reddit.com/r/programming", nil, set, "h"},

		{"android-app://com.example.android", "com.example.android", nil, nil, "o"},
	}

	ctx := gctest.DB(t)

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			h := Hit{Ref: tt.in}
			h.RefURL, _ = url.Parse(tt.in)
			h.Defaults(ctx, false)

			if tt.wantOriginal != nil && *tt.wantOriginal == "_" {
				tt.wantOriginal = &tt.in
			}

			if h.Ref != tt.wantRef {
				t.Fatalf("wrong Ref\nout:  %#v\nwant: %#v\n",
					h.Ref, tt.wantRef)
			}
			if ztype.Deref(h.RefScheme, "") != tt.wantScheme {
				t.Fatalf("wrong RefScheme\nout:  %#v\nwant: %#v\n",
					ztype.Deref(h.RefScheme, ""), tt.wantScheme)
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
		{"", ""},

		{"/page?q=a", "/page?q=a"},
		{"/page?fbclid=foo", "/page"},
		{"/page/?fbclid=foo", "/page"},
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

	ctx := gctest.DB(t)

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			h := Hit{Path: tt.in}
			h.Defaults(ctx, false)

			if h.Path != tt.wantPath {
				t.Fatalf("wrong Path\nout:  %#v\nwant: %#v\n",
					h.Path, tt.wantPath)
			}
		})
	}
}

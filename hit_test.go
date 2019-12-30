// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"zgo.at/zhttp/ctxkey"
	"zgo.at/ztest"
)

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

	ctx := context.WithValue(context.Background(), ctxkey.Site, &Site{ID: 1})

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			h := Hit{Ref: tt.in}
			h.refURL, _ = url.Parse(tt.in)
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

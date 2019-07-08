package goatcounter

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/teamwork/test"
	"zgo.at/zhttp/ctxkey"
)

func TestHitDefaults(t *testing.T) {
	a := "https://arp242.net"
	set := test.SP("_")

	tests := []struct {
		in           string
		wantRef      string
		wantParams   *string
		wantOriginal *string
	}{
		// Split out query parameters.
		{"https://arp242.net", a, nil, nil},
		{"https://arp242.net?a=b", a, test.SP("a=b"), nil},
		{"https://arp242.net?a=b&c=d", a, test.SP("a=b&c=d"), nil},

		// Clean up query parameters.
		{"https://t.co/asd?amp=1", "https://t.co/asd", nil, nil},
		{"https://arp242.net?utm_source=asd", a, nil, set},
		{"https://arp242.net?utm_source=asd&a=b", a, test.SP("a=b"), set},

		// Groups
		{"https://mail.google.com?a=b&c=d", "Gmail", nil, set},
		{"android-app://com.laurencedawson.reddit_sync.pro", "www.reddit.com", nil, set},

		// Host aliases.
		{"https://en.m.wikipedia.org/wiki/Foo", "https://en.wikipedia.org/wiki/Foo", nil, set},
		{"https://en.m.wikipedia.org/wiki/Foo?a=b", "https://en.wikipedia.org/wiki/Foo", test.SP("a=b"), set},
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

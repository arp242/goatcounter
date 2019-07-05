package goatcounter

import (
	"context"
	"net/url"
	"testing"

	"github.com/teamwork/test"
	"zgo.at/zhttp/ctxkey"
)

func TestDefaults(t *testing.T) {
	tests := []struct {
		in   string
		want *string
	}{
		{"https://arp242.net", nil},
		{"https://arp242.net?a=b", test.SP("a=b")},
		{"https://arp242.net?a=b&c=d", test.SP("a=b&c=d")},

		{"https://mail.google.com?a=b&c=d", nil},
		{"https://arp242.net?amp=1", nil},
		{"https://arp242.net?utm_source=asd", nil},
		{"https://arp242.net?utm_source=asd&a=b", test.SP("a=b")},
	}

	ctx := context.WithValue(context.Background(), ctxkey.Site, &Site{ID: 1})

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			h := Hit{Ref: tt.in}
			h.refURL, _ = url.Parse(tt.in)
			h.Defaults(ctx)
			out := h.RefParams
			if out == nil || tt.want == nil {
				if out == nil && tt.want != nil {
					t.Fatalf("\nout:  %#v\nwant: %#v\n", out, *tt.want)
				}
				if tt.want == nil && out != nil {
					t.Fatalf("\nout:  %#v\nwant: %#v\n", *out, tt.want)
				}
				return
			}

			if *out != *tt.want {
				t.Errorf("\nout:  %#v\nwant: %#v\n", *out, *tt.want)
			}
		})
	}
}

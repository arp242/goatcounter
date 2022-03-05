// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"testing"
)

func TestRefspam(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"notinthelist.com", false},
		{"foo.notinthelist.com", false},

		{"localhost", true},
		{"a.localhost", true},
		{"c.a.localhost", true},

		{"adcash.com", true},
		{"d.adcash.com", true},

		{"dadcash.com", false},
		{"localhost.com", false},
		{"asdlocalhost.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := isRefspam(tt.in)
			if got != tt.want {
				t.Errorf("\ngot:  %t\nwant: %t", got, tt.want)
			}
		})
	}
}

func BenchmarkRefspam(b *testing.B) {
	isRefspam("notinthelist.com") // Run the sync.Once

	b.ReportAllocs()
	b.ResetTimer()
	v := false
	for n := 0; n < b.N; n++ {
		v = isRefspam("notinthelist.com")
	}
	_ = v
}

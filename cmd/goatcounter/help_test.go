// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	tests := []struct {
		in      []string
		wantLen int
	}{
		{[]string{"help"}, 5},
		{[]string{"help", "version"}, 2},
		{[]string{"help", "all"}, 50},
	}

	for _, tt := range tests {
		out, code := run(t, "", tt.in)
		if code != 0 {
			t.Fatalf("code is %d: %s", code, strings.Join(out, "\n"))
		}
		if len(out) < tt.wantLen {
			t.Errorf("len too short: %d\n%s", len(out), strings.Join(out, "\n"))
		}
	}
}

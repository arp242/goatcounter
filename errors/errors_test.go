// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		in   error
		want string
	}{
		{Wrap(nil, "nil"), "<nil>"},
		{Wrap(errors.New("e"), ""), ": e"},
		{Wrap(errors.New("a"), "b"), "b: a"},
		{Wrap(fmt.Errorf("b: %w", errors.New("c")), "a"), "a: b: c"},

		{Wrapf(nil, "nil"), "<nil>"},
		{Wrapf(errors.New("e"), ""), ": e"},
		{Wrapf(errors.New("a"), "b"), "b: a"},
		{Wrapf(fmt.Errorf("b: %w", errors.New("c")), "a"), "a: b: c"},

		{Wrapf(errors.New("e"), "fmt: %q, %q", "X", "Y"), `fmt: "X", "Y": e`},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s", tt.in), func(t *testing.T) {
			//out := Wrap(tt.in)
			out := fmt.Sprintf("%v", tt.in)
			if out != tt.want {
				t.Errorf("\nout:  %s\nwant: %s", out, tt.want)
			}
		})
	}
}

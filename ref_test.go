// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
)

func TestListRefsByPath(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.org"},
		Hit{Path: "/y", Ref: "http://example.org"})

	start := time.Now().UTC().Add(-1 * time.Hour)
	end := time.Now().UTC().Add(1 * time.Hour)

	var s Stats
	err := s.ListRefsByPath(ctx, "/x", start, end, 0)
	if err != nil {
		t.Fatal(err)
	}

	got := fmt.Sprintf("%v", s)
	got = regexp.MustCompile(`0x[0-9a-f]{6,}`).ReplaceAllString(got, "0xaa")
	want := `{false [{ example.org 1 0 0xaa} { example.com 2 0 0xaa}]}`

	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

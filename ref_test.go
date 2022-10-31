// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestListRefsByPath(t *testing.T) {
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.org"},
		Hit{Path: "/y", Ref: "http://example.org"})

	rng := ztime.NewRange(ztime.Now().Add(-1 * time.Hour)).To(ztime.Now().Add(1 * time.Hour))

	var s HitStats
	err := s.ListRefsByPath(ctx, "/x", rng, 10, 0)
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

func TestListTopRefs(t *testing.T) {
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/x", Ref: "http://example.com", FirstVisit: true},
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.org"},
		Hit{Path: "/y", Ref: "http://example.org", FirstVisit: true},
		Hit{Path: "/x", Ref: "http://example.org"})

	rng := ztime.NewRange(ztime.Now().Add(-1 * time.Hour)).To(ztime.Now().Add(1 * time.Hour))

	{
		var s HitStats
		err := s.ListTopRefs(ctx, rng, nil, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		got := string(zjson.MustMarshalIndent(s, "\t\t", "\t"))
		want := `
		{
			"more": false,
			"stats": [
				{
					"name": "example.com",
					"count": 2,
					"count_unique": 1,
					"ref_scheme": "h"
				},
				{
					"name": "example.org",
					"count": 3,
					"count_unique": 1,
					"ref_scheme": "h"
				}
			]
		}`
		if d := ztest.Diff(got, want); d != "" {
			t.Error(d)
		}
	}

	{
		var s HitStats
		err := s.ListTopRefs(ctx, rng, []int64{2}, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		got := string(zjson.MustMarshalIndent(s, "\t\t", "\t"))
		want := `
		{
			"more": false,
			"stats": [
				{
					"name": "example.org",
					"count": 1,
					"count_unique": 1,
					"ref_scheme": "h"
				}
			]
		}`
		if d := ztest.Diff(got, want); d != "" {
			t.Error(d)
		}
	}
}

package goatcounter_test

import (
	"testing"
	"time"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
)

func TestListRefsByPathID(t *testing.T) {
	ctx := gctest.DB(t)

	gctest.StoreHits(ctx, t, false,
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.com"},
		Hit{Path: "/x", Ref: "http://example.org"},
		Hit{Path: "/y", Ref: "http://example.org"})

	rng := ztime.NewRange(ztime.Now().Add(-1 * time.Hour)).To(ztime.Now().Add(1 * time.Hour))

	var have HitStats
	err := have.ListRefsByPathID(ctx, 1, rng, 10, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `{
		"more": false,
		"stats": [{
			"count": 0,
			"name": "example.com",
			"ref_scheme": "h"
		}, {
			"count": 0,
			"name": "example.org",
			"ref_scheme": "h"
		}]}`
	if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
		t.Error(d)
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
		var have HitStats
		err := have.ListTopRefs(ctx, rng, nil, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"more": false,
			"stats": [{
				"name": "example.com",
				"count": 1,
				"ref_scheme": "h"
			}, {
				"name": "example.org",
				"count": 1,
				"ref_scheme": "h"
			}]
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}

	{
		var have HitStats
		err := have.ListTopRefs(ctx, rng, []int64{2}, 10, 0)
		if err != nil {
			t.Fatal(err)
		}

		want := `{
			"more": false,
			"stats": [{
				"name": "example.org",
				"count": 1,
				"ref_scheme": "h"
			}]
		}`
		if d := ztest.Diff(zjson.MustMarshalString(have), want, ztest.DiffJSON); d != "" {
			t.Error(d)
		}
	}
}

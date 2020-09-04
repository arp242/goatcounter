package goatcounter_test

import (
	"testing"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zdb"
)

func TestPathsUpdateTitle(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	wantTitle := func(want string) {
		var got string
		err := zdb.MustGet(ctx).GetContext(ctx, &got, `select title from paths limit 1`)
		if err != nil {
			t.Fatal(err)
		}

		if want != got {
			t.Errorf("want: %q, got: %q", want, got)
		}
	}

	p := Path{Path: "/x", Title: "original"}
	err := p.GetOrInsert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantTitle("original")

	for i := 0; i < 10; i++ {
		p2 := Path{Path: "/x", Title: "new"}
		err := p2.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if p2.ID != p.ID {
			t.Fatalf("wrong ID: %d", p2.ID)
		}
		wantTitle("original")
	}

	p2 := Path{Path: "/x", Title: "new"}
	err = p2.GetOrInsert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p2.ID != p.ID {
		t.Fatalf("wrong ID: %d", p2.ID)
	}
	wantTitle("new")
}

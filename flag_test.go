// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"testing"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zdb"
)

func TestHasFlag(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	ctx1, _ := gctest.Site(ctx, t, Site{})
	ok := HasFlag(ctx1, "test")
	if want := false; ok != want {
		t.Errorf("want %t; got %t", want, ok)
	}

	ctx2, site2 := gctest.Site(ctx, t, Site{})
	ok = HasFlag(ctx2, "test")
	if want := false; ok != want {
		t.Errorf("want %t; got %t", want, ok)
	}

	_, err := zdb.MustGet(ctx).ExecContext(ctx, `insert into flags values ('test', $1), ('testall', 0)`, site2.ID)
	if err != nil {
		t.Fatal(err)
	}

	ok = HasFlag(ctx1, "test")
	if want := false; ok != want {
		t.Errorf("want %t; got %t", want, ok)
	}
	ok = HasFlag(ctx1, "testall")
	if want := true; ok != want {
		t.Errorf("want %t; got %t", want, ok)
	}

	ok = HasFlag(ctx2, "test")
	if want := true; ok != want {
		t.Errorf("want %t; got %t", want, ok)
	}
	ok = HasFlag(ctx2, "testall")
	if want := true; ok != want {
		t.Errorf("want %t; got %t", want, ok)
	}
}

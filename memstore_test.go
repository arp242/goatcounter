// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package goatcounter

import (
	"context"
	"testing"

	"zgo.at/zdb"
)

func TestMemstore(t *testing.T) {
	ctx, clean := StartTest(t)
	defer clean()

	for i := 0; i < 2000; i++ {
		Memstore.Append(gen(ctx))
	}

	err := Memstore.Persist(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	err = zdb.MustGet(ctx).GetContext(ctx, &count, `select count(*) from hits`)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2000 {
		t.Errorf("wrong count; wanted 2000 but got %d", count)
	}
}

func gen(ctx context.Context) Hit {
	s := MustGetSite(ctx)
	return Hit{
		Site:    s.ID,
		Path:    "/test",
		Ref:     "https://example.com/test",
		Browser: "test",
	}
}

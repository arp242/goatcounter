// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"context"
	"strings"
	"testing"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zdb"
)

func TestMemstore(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	for i := 0; i < 2000; i++ {
		Memstore.Append(gen(ctx))
	}

	_, err := Memstore.Persist(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	err = zdb.Get(ctx, &count, `select count(*) from hits`)
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
		Site:            s.ID,
		Session:         TestSession,
		Path:            "/test",
		Ref:             "https://example.com/test",
		UserAgentHeader: "test",
	}
}

func TestNextUUID(t *testing.T) {
	want := `11223344556677-8899aabbccddef01
11223344556677-8899aabbccddef02
11223344556677-8899aabbccddef03
11223344556677-8899aabbccddeeff`

	func() {
		_, clean := gctest.DB(t)
		defer clean()

		got := Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			TestSession.Format(16)
		if got != want {
			t.Errorf("wrong:\n%s", got)
		}
	}()

	func() {
		_, clean := gctest.DB(t)
		defer clean()

		got := Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			TestSession.Format(16)
		if got != want {
			t.Errorf("wrong after reset:\n%s", got)
		}
	}()
}

func TestCollect(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()
	clean2 := gctest.SwapNow(t, "2020-06-18")
	defer clean2()

	ctx, site := gctest.Site(ctx, t, Site{Settings: SiteSettings{Collect: 1}})

	h := Hit{
		Site:     site.ID,
		Path:     "/test",
		Ref:      "https://example.com",
		Location: "NL",
		Size:     Floats{5, 6, 7},
	}
	gctest.StoreHits(ctx, t, false, h)

	out := strings.TrimSpace(zdb.DumpString(ctx, `select * from hits`))
	want := strings.TrimSpace(`
hit_id  site_id  path_id  user_agent_id  session                           bot  ref  ref_scheme  size  location  first_visit  created_at
1       2        1        NULL           00112233445566778899aabbccddeeff  0         NULL                        0            2020-06-18 12:00:00`)

	if out != want {
		t.Error(out)
	}

}

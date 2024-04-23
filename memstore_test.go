// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"context"
	"testing"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/ztime"
)

func TestMemstore(t *testing.T) {
	ctx := gctest.DB(t)
	var site Site
	site.Defaults(ctx)
	site.Settings.Collect.Set(CollectHits)
	ctx = gctest.Site(ctx, t, &site, nil)
	ctx = WithSite(ctx, &site)

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
		t.Errorf("wrong count; wanted 2000 but have %d", count)
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

	t.Run("", func(t *testing.T) {
		gctest.DB(t)

		have := Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			TestSession.Format(16)
		if have != want {
			t.Errorf("wrong:\n%s", have)
		}
	})

	t.Run("", func(t *testing.T) {
		gctest.DB(t)

		have := Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			Memstore.SessionID().Format(16) + "\n" +
			TestSession.Format(16)
		if have != want {
			t.Errorf("wrong after reset:\n%s", have)
		}
	})
}

func TestMemstoreCollect(t *testing.T) {
	all := func() zint.Bitflag16 {
		s := SiteSettings{}
		s.Defaults(context.Background())
		s.Collect.Set(CollectHits)
		return s.Collect
	}()

	tests := []struct {
		collect        zint.Bitflag16
		collectRegions Strings
		want           string
	}{
		{all, Strings{}, `
			session                           bot  path    ref          ref_scheme  size     location  first_visit
			00112233445566778899aabbccddeeff  0    /test   example.com  h           5,6,7.0  NL        0
			00112233445566778899aabbccddeeff  0    /other  xxx          c           5,6,7.0  ID-BA     1
		`},

		{CollectNothing, Strings{}, `
			session  bot  path  ref  ref_scheme  size  location  first_visit
		`},

		{all ^ CollectLocationRegion, Strings{}, `
			session                           bot  path    ref          ref_scheme  size     location  first_visit
			00112233445566778899aabbccddeeff  0    /test   example.com  h           5,6,7.0  NL        0
			00112233445566778899aabbccddeeff  0    /other  xxx          c           5,6,7.0  ID        1
		`},

		{all, Strings{"US"}, `
			session                           bot  path    ref          ref_scheme  size     location  first_visit
			00112233445566778899aabbccddeeff  0    /test   example.com  h           5,6,7.0  NL        0
			00112233445566778899aabbccddeeff  0    /other  xxx          c           5,6,7.0  ID        1
		`},
		{all, Strings{"ID"}, `
			session                           bot  path    ref          ref_scheme  size     location  first_visit
			00112233445566778899aabbccddeeff  0    /test   example.com  h           5,6,7.0  NL        0
			00112233445566778899aabbccddeeff  0    /other  xxx          c           5,6,7.0  ID-BA     1
		`},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctx := gctest.DB(t)
			ztime.SetNow(t, "2020-06-18")

			site := Site{Settings: SiteSettings{
				Collect:        tt.collect,
				CollectRegions: tt.collectRegions,
			}}
			ctx = gctest.Site(ctx, t, &site, nil)

			gctest.StoreHits(ctx, t, false, Hit{
				Site:     site.ID,
				Path:     "/test",
				Ref:      "https://example.com",
				Location: "NL",
				Size:     Floats{5, 6, 7},
			}, Hit{
				Site:       site.ID,
				Path:       "/other",
				Query:      "ref=xxx",
				Location:   "ID-BA",
				Size:       Floats{5, 6, 7},
				FirstVisit: true,
			})

			have := zdb.DumpString(ctx, `
				select session, bot, paths.path, refs.ref, refs.ref_scheme, sizes.size, location, first_visit
				from hits
				join paths using (path_id)
				left join refs  using (ref_id)
				left join sizes using (size_id)
			`)
			if d := zdb.Diff(have, tt.want); d != "" {
				t.Error(d)
			}
		})
	}
}

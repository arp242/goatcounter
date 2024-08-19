// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"regexp"
	"strings"
	"testing"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zli"
	"zgo.at/zstd/ztime"
)

func TestDBSchema(t *testing.T) {
	exit, _, out := zli.Test(t)

	runCmd(t, exit, "db", "schema-sqlite")
	wantExit(t, exit, out, 0)
	if len(out.String()) < 1_000 {
		t.Error(out.String())
	}
	out.Reset()

	runCmd(t, exit, "db", "schema-pgsql")
	wantExit(t, exit, out, 0)
	if len(out.String()) < 1_000 {
		t.Error(out.String())
	}
}

func TestDBTest(t *testing.T) {
	exit, _, out, _, dbc := startTest(t)

	runCmd(t, exit, "db", "test", "-db="+dbc)
	wantExit(t, exit, out, 0)
	if !strings.Contains(out.String(), "seems okay") {
		t.Error(out.String())
	}
	out.Reset()

	runCmd(t, exit, "db", "test", "-db=sqlite+yeah_nah_doesnt_exist")
	wantExit(t, exit, out, 1)
	if !strings.Contains(out.String(), `doesn't exist`) {
		t.Error(out.String())
	}
}

func TestDBQuery(t *testing.T) {
	exit, _, out, ctx, dbc := startTest(t)
	ztime.SetNow(t, "2020-06-18")

	runCmd(t, exit, "db", "query", "-db="+dbc, "select site_id, code from sites order by site_id")
	wantExit(t, exit, out, 0)

	want := `
		site_id  code
		1        gctest`
	if d := zdb.Diff(out.String(), want); d != "" {
		t.Error(d)
	}
	out.Reset()

	gctest.StoreHits(ctx, t, false, goatcounter.Hit{
		FirstVisit:      true,
		UserAgentHeader: "Mozilla/5.0 (X11; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0",
	})
}

func TestDBNewDB(t *testing.T) {
	exit, _, out, _, dbc := startTest(t)

	runCmd(t, exit, "db", "newdb", "-db="+dbc)
	wantExit(t, exit, out, 2)

	tmp := t.TempDir()
	dbc = "sqlite3+" + tmp + "/new"

	runCmd(t, exit, "db", "newdb", "-db="+dbc)
	wantExit(t, exit, out, 0)

	runCmd(t, exit, "db", "newdb", "-db="+dbc)
	wantExit(t, exit, out, 2)
}

func TestDBMigrate(t *testing.T) {
	exit, _, out, _, dbc := startTest(t)

	runCmd(t, exit, "db", "migrate", "-db="+dbc, "pending")
	wantExit(t, exit, out, 0)
	want := "no pending migrations\n"
	if out.String() != want {
		t.Error(out.String())
	}
}

func TestDBSite(t *testing.T) {
	exit, _, out, ctx, dbc := startTest(t)

	{ // create
		runCmd(t, exit, "db", "create", "site",
			"-db="+dbc,
			"-vhost=stats.stats",
			"-user.email=foo@foo.foo",
			"-user.password=password")
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select site_id, parent, cname from sites order by site_id`) +
			zdb.DumpString(ctx, `select user_id, site_id, email from users order by user_id`)
		want := `
			site_id  parent  cname
			1        NULL    gctest.localhost
			2        NULL    stats.stats
			user_id  site_id  email
			1        1        test@gctest.localhost
			2        2        foo@foo.foo`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // create with parent
		runCmd(t, exit, "db", "create", "site",
			"-db="+dbc,
			"-link=1",
			"-vhost=stats2.stats")
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select site_id, parent, cname from sites order by site_id`) +
			zdb.DumpString(ctx, `select user_id, site_id, email from users order by user_id`)
		want := `
			site_id  parent  cname
			1        NULL    gctest.localhost
			2        NULL    stats.stats
			3        1       stats2.stats
			user_id  site_id  email
			1        1        test@gctest.localhost
			2        2        foo@foo.foo`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // update
		runCmd(t, exit, "db", "update", "site",
			"-db="+dbc,
			"-find=2",
			"-vhost=update.example.com",
			"-link=1",
		)
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select site_id, parent, cname from sites order by site_id`) +
			zdb.DumpString(ctx, `select user_id, site_id, email from users order by user_id`)
		want := `
			site_id  parent  cname
			1        NULL    gctest.localhost
			2        1       update.example.com
			3        1       stats2.stats
			user_id  site_id  email
			1        1        test@gctest.localhost
			2        1        foo@foo.foo`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // show
		runCmd(t, exit, "db", "show", "site",
			"-db="+dbc,
			"-find=1", "-find=update.example.com")
		wantExit(t, exit, out, 0)
		if !grep(out.String(), `site_id +1`) {
			t.Error(out.String())
		}
		if !grep(out.String(), `site_id +2`) {
			t.Error(out.String())
		}
		out.Reset()
	}

	{ // delete when site still has children
		runCmd(t, exit, "db", "delete", "site",
			"-db="+dbc,
			"-find=1",
		)
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select site_id, parent, cname, state from sites order by site_id`) +
			zdb.DumpString(ctx, `select user_id, site_id, email from users order by user_id`)
		want := `
			site_id  parent  cname               state
			1        NULL    NULL                d
			2        NULL    update.example.com  a
			3        2       stats2.stats        a
			user_id  site_id  email
			1        2        test@gctest.localhost
			2        2        foo@foo.foo`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // delete
		runCmd(t, exit, "db", "delete", "site",
			"-db="+dbc,
			"-find=3",
		)
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select site_id, parent, cname, state from sites order by site_id`) +
			zdb.DumpString(ctx, `select user_id, site_id, email from users order by user_id`)
		want := `
			site_id  parent  cname               state
			1        NULL    NULL                d
			2        NULL    update.example.com  a
			3        2       NULL                d
			user_id  site_id  email
			1        2        test@gctest.localhost
			2        2        foo@foo.foo`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
	}
}

func grep(s, find string) bool {
	return regexp.MustCompile(find).MatchString(s)
}

func TestDBUser(t *testing.T) {
	exit, _, out, ctx, dbc := startTest(t)

	{ // create
		runCmd(t, exit, "db", "create", "user",
			"-db="+dbc,
			"-site=1",
			"-email=foo@foo.foo",
			"-password=password",
			"-access=readonly")
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select user_id, site_id, email, access from users order by user_id`)
		want := `
			user_id  site_id  email                  access
			1        1        test@gctest.localhost  {"all":"a"}
			2        1        foo@foo.foo            {"all":"r"}`
		if pgSQL {
			want = strings.ReplaceAll(want, `":"`, `": "`)
		}
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // update
		runCmd(t, exit, "db", "update", "user",
			"-db="+dbc,
			"-find=2",
			"-email=new@new.new",
			"-password=password",
			"-access=settings")
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select user_id, site_id, email, access from users order by user_id`)
		want := `
			user_id  site_id  email                  access
			1        1        test@gctest.localhost  {"all":"a"}
			2        1        new@new.new            {"all":"s"}`
		if pgSQL {
			want = strings.ReplaceAll(want, `":"`, `": "`)
		}
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // show
		runCmd(t, exit, "db", "show", "user",
			"-db="+dbc,
			"-find=1", "-find=new@new.new")
		wantExit(t, exit, out, 0)
		if r := `user_id +1`; !grep(out.String(), r) {
			t.Errorf("user 1 not found in output (via regexp %q):\n%s", r, out.String())
		}
		if r := `user_id +2`; !grep(out.String(), r) {
			t.Errorf("user 2 not found in output (via regexp %q):\n%s", r, out.String())
		}
		out.Reset()
	}

	{ // delete
		runCmd(t, exit, "db", "delete", "user",
			"-db="+dbc,
			"-find=2",
		)
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select user_id, site_id, email, access from users order by user_id`)
		want := `
			user_id  site_id  email                  access
			1        1        test@gctest.localhost  {"all":"a"}`
		if pgSQL {
			want = strings.ReplaceAll(want, `":"`, `": "`)
		}
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // delete when last admin
		runCmd(t, exit, "db", "delete", "user",
			"-db="+dbc,
			"-find=1",
		)
		wantExit(t, exit, out, 1)

		have := zdb.DumpString(ctx, `select user_id, site_id, email, access from users order by user_id`)
		want := `
			user_id  site_id  email                  access
			1        1        test@gctest.localhost  {"all":"a"}`
		if pgSQL {
			want = strings.ReplaceAll(want, `":"`, `": "`)
		}
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // force delete
		runCmd(t, exit, "db", "delete", "user",
			"-db="+dbc,
			"-find=1",
			"-force",
		)
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select user_id, site_id, email, access from users order by user_id`)
		want := `
			user_id  site_id  email  access`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}
}

func TestDBAPIToken(t *testing.T) {
	exit, _, out, ctx, dbc := startTest(t)

	{ // create
		runCmd(t, exit, "db", "create", "apitoken",
			"-db="+dbc,
			"-user=1",
			"-name=abc def",
			"-perm=count,export,site_read,site_create,site_update")
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select api_token_id, site_id, user_id, name, permissions from api_tokens order by api_token_id`)
		want := `
			api_token_id  site_id  user_id  name     permissions
			1             1        1        abc def  62`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // update
		runCmd(t, exit, "db", "update", "apitoken",
			"-db="+dbc,
			"-find=1",
			"-name=new",
			"-perm=count")
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select api_token_id, site_id, user_id, name, permissions from api_tokens order by api_token_id`)
		want := `
			api_token_id  site_id  user_id  name  permissions
			1             1        1        new   2`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
		out.Reset()
	}

	{ // show
		runCmd(t, exit, "db", "show", "apitoken",
			"-db="+dbc,
			"-find=1")
		wantExit(t, exit, out, 0)
		if !strings.HasPrefix(out.String(), `api_token_id  1`) {
			t.Error(out.String())
		}
		out.Reset()
	}

	{ // delete
		runCmd(t, exit, "db", "delete", "apitoken",
			"-db="+dbc,
			"-find=1",
		)
		wantExit(t, exit, out, 0)

		have := zdb.DumpString(ctx, `select api_token_id, site_id, user_id, name, permissions from api_tokens order by api_token_id`)
		want := `
			api_token_id  site_id  user_id  name  permissions`
		if d := zdb.Diff(have, want); d != "" {
			t.Error(d)
		}
	}
}

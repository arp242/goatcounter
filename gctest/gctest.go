// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package gctest contains testing helpers.
package gctest

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
)

var pgSQL = false

type tester interface {
	Helper()
	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Logf(string, ...interface{})
}

func init() {
	sql.Register("sqlite3_zdb", &sqlite3.SQLiteDriver{
		ConnectHook: goatcounter.SQLiteHook,
	})
}

func Reset() {
	goatcounter.Memstore.Reset()
	goatcounter.Reset()
}

// DB starts a new database test.
func DB(t tester) (context.Context, func()) {
	t.Helper()

	cfg.RunningTests = true
	dbname := "goatcounter_test_" + zcrypto.Secret64()

	var (
		db  zdb.DBCloser
		err error
	)
	if pgSQL {
		out, err2 := exec.Command("createdb", dbname).CombinedOutput()
		if err2 != nil {
			t.Fatalf("%s → %s", err2, out)
		}

		os.Setenv("PGDATABASE", dbname)
		db, err = zdb.Connect(zdb.ConnectOptions{
			Connect: "postgresql://",
		})
	} else {
		db, err = zdb.Connect(zdb.ConnectOptions{
			Connect: "sqlite3://:memory:?cache=shared",
		})
	}
	if err != nil {
		t.Fatalf("connect to DB: %s", err)
	}

	ctx := zdb.With(context.Background(), db)
	setupDB(t, db)
	goatcounter.Memstore.TestInit(db)
	ctx = initData(ctx, db, t)

	return ctx, func() {
		goatcounter.Memstore.Reset()
		goatcounter.Reset()
		db.Close()
		if zdb.PgSQL(db) {
			exec.Command("dropdb", dbname).Run()
		}
	}
}

func setupDB(t tester, db zdb.DB) {
	top, err := os.Getwd()
	if err != nil {
		t.Fatalf(fmt.Sprintf("cannot get cwd: %s", err))
	}
	for {
		if filepath.Base(top) == "goatcounter" {
			break
		}
		top = filepath.Dir(top)
		// Hit root path, I don't know how that will appear on Windows so check
		// the len(). Should never happen anyway.
		if len(top) < 5 {
			break
		}
	}
	schemapath := top + "/db/schema.sql"
	migratepath := top + "/db/migrate/sqlite"
	if pgSQL {
		schemapath = top + "/db/schema.pgsql"
		migratepath = top + "/db/migrate/pgsql"
	}

	s, err := ioutil.ReadFile(schemapath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema := string(s)
	_, err = db.ExecContext(context.Background(), schema)
	if err != nil {
		t.Fatalf("run schema %q: %v", schemapath, err)
	}

	migs, err := ioutil.ReadDir(migratepath)
	if err != nil {
		t.Fatalf("read migration directory: %s", err)
	}

	var migrations [][]string
	for _, m := range migs {
		if !strings.HasSuffix(m.Name(), ".sql") {
			continue
		}
		var ran bool
		db.GetContext(context.Background(), &ran, `select 1 from version where name=$1`, m.Name()[:len(m.Name())-4])
		if ran {
			continue
		}

		mp := fmt.Sprintf("%s/%s", migratepath, m.Name())
		mb, err := ioutil.ReadFile(mp)
		if err != nil {
			t.Fatalf("read migration: %s", err)
		}
		migrations = append(migrations, []string{mp, string(mb)})
	}

	for _, m := range migrations {
		_, err = db.ExecContext(context.Background(), m[1])
		if err != nil {
			t.Fatalf("run migration %q: %s", m[0], err)
		}
	}
}

func initData(ctx context.Context, db zdb.DB, t tester) context.Context {
	site := goatcounter.Site{Code: "gctest", Plan: goatcounter.PlanPersonal}
	err := site.Insert(ctx)
	if err != nil {
		t.Fatalf("create site: %s", err)
	}
	ctx = goatcounter.WithSite(ctx, &site)

	user := goatcounter.User{Site: site.ID, Email: "test@example.com", Password: []byte("coconuts")}
	err = user.Insert(ctx)
	if err != nil {
		t.Fatalf("create user: %s", err)
	}
	ctx = goatcounter.WithUser(ctx, &user)

	return ctx
}

// StoreHits is a convenient helper to store hits in the DB via Memstore and
// cron.UpdateStats().
func StoreHits(ctx context.Context, t *testing.T, wantFail bool, hits ...goatcounter.Hit) []goatcounter.Hit {
	t.Helper()

	for i := range hits {
		if hits[i].Session.IsZero() {
			hits[i].Session = goatcounter.TestSession
		}
		if hits[i].Site == 0 {
			hits[i].Site = 1
		}
		if hits[i].Path == "" {
			hits[i].Path = "/"
		}
	}

	goatcounter.Memstore.Append(hits...)
	hits, err := goatcounter.Memstore.Persist(ctx)
	if !wantFail && err != nil {
		t.Fatalf("gctest.StoreHits failed: %s", err)
	}
	if wantFail && err == nil {
		t.Fatal("gc.StoreHits: no error while wantError is true")
	}

	sites := make(map[int64]struct{})
	for _, h := range hits {
		sites[h.Site] = struct{}{}
	}

	for s := range sites {
		err = cron.UpdateStats(ctx, nil, s, hits, false)
		if err != nil {
			t.Fatal(err)
		}
	}

	return hits
}

func Site(ctx context.Context, t *testing.T, site goatcounter.Site) (context.Context, goatcounter.Site) {
	if site.Code == "" {
		site.Code = zcrypto.Secret64()
		if len(site.Code) > 50 {
			site.Code = site.Code[:50]
		}
	}
	if site.Plan == "" {
		site.Plan = goatcounter.PlanPersonal
	}

	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx = goatcounter.WithSite(ctx, &site)

	user := goatcounter.User{
		Site:     site.ID,
		Email:    "test@example.com",
		Password: []byte("coconuts"),
	}
	err = user.Insert(ctx)
	if err != nil {
		t.Fatalf("get/create user: %s", err)
	}
	ctx = goatcounter.WithUser(ctx, &user)

	return ctx, site
}

func SwapNow(t *testing.T, date interface{}) func() {
	var (
		d   time.Time
		err error
	)
	switch dd := date.(type) {
	case string:
		if len(dd) == 10 {
			dd += " 12:00:00"
		}
		d, err = time.Parse("2006-01-02 15:04:05", dd)
	case time.Time:
		d = dd
	case *time.Time:
		d = *dd
	default:
		t.Fatalf("unknown type: %T", date)
	}
	if err != nil {
		t.Fatal(err)
	}

	goatcounter.Now = func() time.Time { return d }
	return func() {
		goatcounter.Now = func() time.Time { return time.Now().UTC() }
	}
}

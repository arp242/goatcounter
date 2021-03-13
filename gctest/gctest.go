// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// Package gctest contains testing helpers.
package gctest

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/zdb"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zgo"
)

var pgSQL = false

func init() {
	sql.Register("sqlite3_zdb", &sqlite3.SQLiteDriver{
		ConnectHook: goatcounter.SQLiteHook,
	})
	goatcounter.InitGeoDB("")
}

func Context(db zdb.DB) context.Context {
	ctx := goatcounter.NewContext(db)
	goatcounter.Config(ctx).BcryptMinCost = true
	goatcounter.Config(ctx).Plan = goatcounter.PlanPersonal
	goatcounter.Config(ctx).Domain = "example.com"
	return ctx
}

func Reset() {
	goatcounter.Memstore.Reset()
}

// DB starts a new database test.
func DB(t testing.TB) (context.Context, func()) {
	t.Helper()
	return db(t, false)
}

// DBFile is like DB(), but guarantees that the database will be written to
// disk, whereas DB() may store it in memory.
//
// You can get the connection string from the GCTEST_CONNECT environment
// variable.
func DBFile(t testing.TB) (context.Context, func()) {
	// TODO: now that we have t.Cleanup() we can use that, instead of returning
	// a function.
	t.Helper()
	return db(t, true)
}

func db(t testing.TB, storeFile bool) (context.Context, func()) {
	t.Helper()

	dbname := "goatcounter_test_" + zcrypto.Secret64()

	conn := "sqlite3://:memory:?cache=shared"
	if storeFile {
		conn = "sqlite://" + t.TempDir() + "/goatcounter.sqlite3"
	}
	if pgSQL {
		os.Setenv("PGDATABASE", dbname)
		conn = "postgresql://"
	}
	os.Setenv("GCTEST_CONNECT", conn)

	db, err := zdb.Connect(zdb.ConnectOptions{
		Connect:      conn,
		Files:        os.DirFS(zgo.ModuleRoot()),
		Migrate:      []string{"all"},
		GoMigrations: gomig.Migrations,
		Create:       true,
		SQLiteHook:   goatcounter.SQLiteHook,
	})
	if err != nil {
		t.Fatalf("connect to DB: %s", err)
	}

	ctx := Context(db)
	goatcounter.Memstore.TestInit(db)
	ctx = initData(ctx, db, t)

	return ctx, func() {
		goatcounter.Memstore.Reset()
		db.Close()
		if db.Driver() == zdb.DriverPostgreSQL {
			exec.Command("dropdb", dbname).Run()
		}
	}
}

func initData(ctx context.Context, db zdb.DB, t testing.TB) context.Context {
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

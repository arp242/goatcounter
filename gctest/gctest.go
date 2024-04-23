// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

// Package gctest contains testing helpers.
package gctest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/text/language"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/db/migrate/gomig"
	"zgo.at/z18n"
	"zgo.at/zdb"
	"zgo.at/zdb/drivers/go-sqlite3"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zgo"
	"zgo.at/zstd/ztype"
)

var pgSQL = false

func init() {
	sqlite3.DefaultHook(goatcounter.SQLiteHook)
	goatcounter.InitGeoDB("")
}

// Context creates a new test context.
func Context(db zdb.DB) context.Context {
	ctx := goatcounter.NewContext(db)
	ctx = z18n.With(ctx, z18n.NewBundle(language.BritishEnglish).Locale("en-GB"))
	goatcounter.Config(ctx).BcryptMinCost = true
	goatcounter.Config(ctx).GoatcounterCom = true
	goatcounter.Config(ctx).Domain = "test"
	return ctx
}

// Reset global state.
func Reset() {
	goatcounter.Memstore.Reset()
}

// DB starts a new database test.
func DB(t testing.TB) context.Context {
	t.Helper()
	return db(t, false)
}

// DBFile is like DB(), but guarantees that the database will be written to
// disk, whereas DB() may store it in memory.
//
// You can get the connection string from the GCTEST_CONNECT environment
// variable.
func DBFile(t testing.TB) context.Context {
	t.Helper()
	return db(t, true)
}

// TODO: this should use zdb.StartTest(); need to be able to pass in some
// zdb.ConnectOptions{} to that though.
// TODO: we can create unlogged tables in PostgreSQL, which should be faster:
//
//	create unlogged table foo [..]
func db(t testing.TB, storeFile bool) context.Context {
	t.Helper()

	dbname := "goatcounter_test_" + zcrypto.Secret64()

	conn := "sqlite3+:memory:?cache=shared"
	if storeFile {
		conn = "sqlite+" + t.TempDir() + "/goatcounter.sqlite3"
	}
	if pgSQL {
		os.Setenv("PGDATABASE", dbname)
		conn = "postgresql+"
	}
	os.Setenv("GCTEST_CONNECT", conn)

	db, err := zdb.Connect(context.Background(), zdb.ConnectOptions{
		Connect:      conn,
		Files:        os.DirFS(zgo.ModuleRoot()),
		Migrate:      []string{"all"},
		GoMigrations: gomig.Migrations,
		Create:       true,
	})
	if err != nil {
		t.Fatalf("connect to DB: %s", err)
	}

	ctx := Context(db)
	goatcounter.Memstore.TestInit(db)
	ctx = initData(ctx, db, t)
	cron.Start(ctx)

	t.Cleanup(func() {
		goatcounter.Memstore.Reset()
		cron.Stop()
		db.Close()

		_, keepdb := os.LookupEnv("KEEPDB")
		switch db.SQLDialect() {
		case zdb.DialectPostgreSQL:
			db.Close()
			if keepdb {
				fmt.Println("KEPT DATABASE")
				fmt.Println("    psql", dbname)
			} else {
				exec.Command("dropdb", dbname).Run()
			}
		default:
			if keepdb {
				fmt.Println("KEEPDB not supported for this SQL dialect")
			}
			db.Close()
		}
	})

	return ctx
}

func initData(ctx context.Context, db zdb.DB, t testing.TB) context.Context {
	site := goatcounter.Site{Code: "gctest", Cname: ztype.Ptr("gctest.localhost")}
	err := site.Insert(ctx)
	if err != nil {
		t.Fatalf("create site: %s", err)
	}
	ctx = goatcounter.WithSite(ctx, &site)

	user := goatcounter.User{
		Site:          site.ID,
		Access:        goatcounter.UserAccesses{"all": goatcounter.AccessAdmin},
		Email:         "test@gctest.localhost",
		EmailVerified: true,
		Password:      []byte("coconuts"),
	}
	err = user.Insert(ctx, false)
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

	siteID := int64(1)
	if s := goatcounter.GetSite(ctx); s != nil {
		siteID = s.ID
	}
	for i := range hits {
		if hits[i].Session.IsZero() {
			hits[i].Session = goatcounter.TestSession
		}
		if hits[i].Site == 0 {
			hits[i].Site = siteID
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
		err = cron.UpdateStats(ctx, nil, s, hits)
		if err != nil {
			t.Fatal(err)
		}
	}

	return hits
}

// Site creates a new user/site pair.
//
// You can set values for the site by passing the sute or user parameters, but
// they may be nil to just set them to some sensible defaults.
func Site(ctx context.Context, t *testing.T, site *goatcounter.Site, user *goatcounter.User) context.Context {
	if site == nil {
		site = &goatcounter.Site{}
	}
	if user == nil {
		user = &goatcounter.User{}
	}

	if site.Code == "" {
		site.Code = "gctest-" + zcrypto.Secret64()
	}

	err := site.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ctx = goatcounter.WithSite(ctx, site)

	user.Site = site.ID
	if user.Email == "" {
		user.Email = "test@example.com"
	}
	if len(user.Password) == 0 {
		user.Password = []byte("coconuts")
	}
	if user.Access == nil {
		user.Access = goatcounter.UserAccesses{"all": goatcounter.AccessAdmin}
	}
	err = user.Insert(ctx, false)
	if err != nil {
		t.Fatalf("get/create user: %s", err)
	}
	ctx = goatcounter.WithUser(ctx, user)

	return ctx
}

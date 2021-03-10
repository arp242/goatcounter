// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"sync"
	_ "time/tzdata"

	_ "github.com/lib/pq"           // PostgreSQL database driver.
	_ "github.com/mattn/go-sqlite3" // SQLite database driver.
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/zdb"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zgo"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zstring"
)

var version = "dev"

func init() {
	errors.Package = "zgo.at/goatcounter"
}

type command func(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error

func main() {
	var (
		f     = zli.NewFlags(os.Args)
		ready = make(chan struct{}, 1)
		stop  = make(chan struct{}, 1)
	)
	cmdMain(f, ready, stop)
}

var mainDone sync.WaitGroup

func cmdMain(f zli.Flags, ready chan<- struct{}, stop chan struct{}) {
	mainDone.Add(1)
	defer mainDone.Done()

	goatcounter.Version = version

	cmd := f.Shift()
	if zstring.ContainsAny(f.Args, "-h", "-help", "--help") {
		f.Args = append([]string{cmd}, f.Args...)
		cmd = "help"
	}

	var run command
	switch cmd {
	default:
		zli.Errorf(usage[""])
		zli.Errorf("unknown command: %q", cmd)
		zli.Exit(1)
		return
	case "", "help":
		run = cmdHelp
	case "version":
		fmt.Fprintln(zli.Stdout, getVersion())
		zli.Exit(0)
		return

	case "db", "database":
		run = cmdDb
	case "create":
		run = cmdCreate
	case "migrate":
		run = cmdMigrate
	case "reindex":
		run = cmdReindex
	case "serve":
		run = cmdServe
	case "saas":
		run = cmdSaas
	case "monitor":
		run = cmdMonitor
	case "import":
		run = cmdImport
	case "buffer":
		run = cmdBuffer
	}

	err := run(f, ready, stop)
	if err != nil {
		zli.Errorf(err)
		zli.Exit(1)
		return
	}
	zli.Exit(0)
}

func connectDB(connect string, migrate []string, create, prod bool) (zdb.DB, context.Context, error) {
	var files fs.FS = goatcounter.DB
	if !prod {
		files = os.DirFS(zgo.ModuleRoot())
	}

	db, err := zdb.Connect(zdb.ConnectOptions{
		Connect:      connect,
		Files:        files,
		Migrate:      migrate,
		GoMigrations: gomig.Migrations,
		Create:       create,
		SQLiteHook:   goatcounter.SQLiteHook,
	})
	var pErr *zdb.PendingMigrationsError
	if errors.As(err, &pErr) {
		zlog.Printf("WARNING: %s", err)
		err = nil
	}
	return db, goatcounter.NewContext(db), err
}

func getVersion() string {
	return fmt.Sprintf("version=%s; go=%s; GOOS=%s; GOARCH=%s; race=%t; cgo=%t",
		version, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		zruntime.Race, zruntime.CGO)
}

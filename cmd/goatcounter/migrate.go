// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"fmt"
	"strings"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/zdb"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zstring"
)

const usageMigrate = `
Run database migrations and exit.

Flags:

  -db          Database connection: "sqlite://<file>" or "postgres://<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite://db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -createdb    Create the database if it doesn't exist yet; only for SQLite.

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

Positional arguments are names of database migrations, either as just the name
("2020-01-05-2-foo") or as the file path ("./db/migrate/sqlite/2020-01-05-2-foo.sql").

Use "all" to run all migrations that haven't been run yet, or "show" to only
display pending migrations.

Note: you can also use -automigrate flag for the serve command to run migrations
on startup.
`

func cmdMigrate(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	defer func() { ready <- struct{}{} }()

	var (
		dbConnect = f.String("sqlite://db/goatcounter.sqlite3", "db").Pointer()
		debug     = f.String("", "debug").Pointer()
		createdb  = f.Bool(false, "createdb").Pointer()
	)
	err := f.Parse()
	if err != nil {
		return err
	}

	if len(f.Args) == 0 {
		return errors.New("need a migration or command")
	}

	return func(dbConnect, debug string, createdb bool) error {
		zlog.Config.SetDebug(debug)

		db, err := connectDB(dbConnect, nil, createdb, true)
		if err != nil {
			return err
		}
		defer db.Close()

		m, err := zdb.NewMigrate(db, goatcounter.DB, gomig.Migrations)
		if err != nil {
			return err
		}

		if zstring.Contains(f.Args, "show") || zstring.Contains(f.Args, "list") {
			have, ran, err := m.List()
			if err != nil {
				return err
			}
			if d := zstring.Difference(have, ran); len(d) > 0 {
				fmt.Fprintf(zli.Stdout, "Pending migrations:\n\t%s\n", strings.Join(d, "\n\t"))
			} else {
				fmt.Fprintln(zli.Stdout, "No pending migrations")
			}
			return nil
		}

		return m.Run(f.Args...)
	}(*dbConnect, *debug, *createdb)
}

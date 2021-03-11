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

Special values:

    all         Run all pending migrations.
    pending     Show pending migrations but do not run anything. Exits with 1 if
                there are pending migrations, or 0 if there aren't.
    list        List all migrations; pending migrations are prefixed with
                "pending: ". Always exits with 0.

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

		db, _, err := connectDB(dbConnect, nil, createdb, false)
		if err != nil {
			return err
		}
		defer db.Close()

		m, err := zdb.NewMigrate(db, goatcounter.DB, gomig.Migrations)
		if err != nil {
			return err
		}

		if zstring.ContainsAny(f.Args, "pending", "list") {
			have, ran, err := m.List()
			if err != nil {
				return err
			}
			diff := zstring.Difference(have, ran)
			pending := "no pending migrations"
			if len(diff) > 0 {
				pending = fmt.Sprintf("pending migrations:\n\t%s", strings.Join(diff, "\n\t"))
			}

			if zstring.Contains(f.Args, "list") {
				for i := range have {
					if zstring.Contains(diff, have[i]) {
						have[i] = "pending: " + have[i]
					}
				}
				fmt.Fprintln(zli.Stdout, strings.Join(have, "\n"))
				return nil
			}

			if len(diff) > 0 {
				return errors.New(pending)
			}
			fmt.Fprintln(zli.Stdout, pending)
			return nil
		}

		return m.Run(f.Args...)
	}(*dbConnect, *debug, *createdb)
}

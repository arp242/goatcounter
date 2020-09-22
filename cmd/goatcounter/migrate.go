// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"fmt"
	"os"
	"strings"

	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/goatcounter/pack"
	"zgo.at/zdb"
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

Positional arguments are names of database migrations, either as just the name
("2020-01-05-2-foo") or as the file path ("./db/migrate/sqlite/2020-01-05-2-foo.sql").

Use "all" to run all migrations that haven't been run yet, or "show" to only
display pending migrations.

Note: you can also use -automigrate flag for the serve command to run migrations
on startup.
`

func migrate() (int, error) {
	if len(os.Args) == 2 {
		return 1, errors.New("need a migration or command")
	}

	dbConnect := flagDB()
	debug := flagDebug()

	var createdb bool
	CommandLine.BoolVar(&createdb, "createdb", false, "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}

	zlog.Config.SetDebug(*debug)

	db, err := connectDB(*dbConnect, CommandLine.Args(), createdb)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	if zstring.Contains(CommandLine.Args(), "show") || zstring.Contains(CommandLine.Args(), "list") {
		m := zdb.NewMigrate(db, []string{"show"},
			map[bool]map[string][]byte{true: pack.MigrationsPgSQL, false: pack.MigrationsSQLite}[cfg.PgSQL],
			gomig.Migrations,
			map[bool]string{true: "db/migrate/pgsql", false: "db/migrate/sqlite"}[cfg.PgSQL])
		have, ran, err := m.List()
		if err != nil {
			return 1, err
		}
		if d := zstring.Difference(have, ran); len(d) > 0 {
			fmt.Fprintf(stdout, "Pending migrations:\n\t%s\n", strings.Join(d, "\n\t"))
		} else {
			fmt.Fprintln(stdout, "No pending migrations")
		}
	}

	return 0, nil
}

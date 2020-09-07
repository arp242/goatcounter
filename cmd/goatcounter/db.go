// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"fmt"
	"os"

	"zgo.at/errors"
	"zgo.at/goatcounter/pack"
	"zgo.at/zdb"
)

const helpDatabase = `
The db command accepts one of three commands;

    schema-sqlite      Print the SQLite database schema.
    schema-pgsql       Print the PostgreSQL database schema.
    test               Test if the database seems to exist; exits with 0 on
                       success, 2 if there's a DB connection error, and 1 on any
                       other error. This requires setting a -db flag.

These are mostly useful for setting up new instances in scripts, e.g.:

    goatcounter db -db [..] test  # Note: -db must come before test!
    if [ $? -eq 2 ]; then
        createdb goatcounter5
        goatcounter db schema-pgsql | psql goatcounter5
    fi

Detailed documentation on the -db flag:

GoatCounter can use SQLite and PostgreSQL. All commands accept the -db flag to
customize the database connection string.

You can select a database engine by using "sqlite://[..]" for SQLite, or
"postgresql://[..]" (or "postgres://[..]") for PostgreSQL.

There are no plans to support other database engines such as MySQL/MariaDB.

SQLite:

    This is the default database engine as it has no dependencies, and for most
    small to medium usage it should be more than fast enough.

    The SQLite connection string is usually just a filename, optionally prefixed
    with "file:". Parameters can be added as a URL query string after a ?:

        -db 'sqlite://mydb.sqlite?param=value&other=value'

    See the go-sqlite3 documentation for a list of supported parameters:
    https://github.com/mattn/go-sqlite3/#connection-string

    _journal_mode=wal is always added unless explicitly overridden. Generally
    speaking using a Write-Ahead-Log is more suitable for GoatCounter than the
    default DELETE journaling.

    The database is automatically created for the "serve" command, but you need
    to add -createdb to any other commands to create the database. This is to
    prevent accidentally operating on the wrong (new) database.

PostgreSQL:

    PostgreSQL provides better performance for large instances. If you have
    millions of pageviews then PostgreSQL is probably a better choice.

    The PostgreSQL connection string can either be as "key=value" or as an URL;
    the following are identical:

        -db 'postgresql://user=pqgotest dbname=pqgotest sslmode=verify-full'
        -db 'postgresql://pqgotest:password@localhost/pqgotest?sslmode=verify-full'

    See the pq documentation for a list of supported parameters:
    https://pkg.go.dev/github.com/lib/pq?tab=doc#hdr-Connection_String_Parameters

    You may want to consider lowering the "seq_page_cost" parameter; the query
    planner tends to prefer seq scans instead of index scans for some operations
    with the default of 4, which is much slower.

    I found that 0.5 is a fairly good setting, you can set it in your
    postgresql.conf file, or just for one database with:

        alter database goatcounter set seq_page_cost=.5

    The database isn't automatically created for PostgreSQL, you'll have to
    manually create it first:

        createdb goatcounter
        psql goatcounter < ./db/schema.pgsql
`

func database() (int, error) {
	dbConnect := CommandLine.String("db", "", "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}
	cmd := CommandLine.Args()

	if len(cmd) == 0 {
		return 1, fmt.Errorf("need a subcommand: schema-sqlite, schema-pgsql, or test")
	}
	switch cmd[0] {
	default:
		return 1, fmt.Errorf("unknown subcommand: %q", os.Args[2])
	case "schema-sqlite":
		fmt.Println(string(pack.SchemaSQLite))
		return 0, nil
	case "schema-pgsql":
		fmt.Println(string(pack.SchemaPgSQL))
		return 0, nil
	case "test":
		if *dbConnect == "" {
			return 1, errors.New("must add -db flag")
		}
		db, err := zdb.Connect(zdb.ConnectOptions{Connect: *dbConnect})
		if err != nil {
			return 2, err
		}
		defer db.Close()
		var i int
		err = db.Get(&i, `select 1 from version`)
		if err != nil {
			return 2, fmt.Errorf("select 1 from version: %w", err)
		}
		fmt.Println("DB seems okay")
	}

	return 0, nil
}

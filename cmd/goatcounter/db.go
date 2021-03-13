// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"fmt"

	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zli"
)

const helpDatabase = `
The db command accepts three commands;

    schema-sqlite      Print the SQLite database schema.
    schema-pgsql       Print the PostgreSQL database schema.
    test               Test if the database seems to exist; exits with 0 on
                       success, 2 if there's a DB connection error, and 1 on any
                       other error. This requires setting a -db flag.

This is useful for setting up new instances in scripts, e.g.:

    goatcounter db -db [..] test  # Note: -db must come before test!
    if [ $? -eq 2 ]; then
        createdb goatcounter
        goatcounter db schema-pgsql | psql goatcounter
    fi

Detailed documentation on the -db flag:

    GoatCounter can use SQLite and PostgreSQL. All commands accept the -db flag
    to customize the database connection string.

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

    _journal_mode=wal is always added unless explicitly overridden. Usually the
    Write Ahead Log is more suitable for GoatCounter than the default DELETE
    journaling.

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

    You can also use the standard PG* environment variables:

        PGDATABASE=goatcounter DBHOST=/var/run goatcounter -db 'postgresql://'

    You may want to consider lowering the "seq_page_cost" parameter; the query
    planner tends to prefer seq scans instead of index scans for some operations
    with the default of 4, which is much slower. I found that 0.5 is a fairly
    good setting, you can set it in your postgresql.conf file, or just for one
    database with:

        alter database goatcounter set seq_page_cost=.5

    The database isn't automatically created for PostgreSQL, you'll have to
    manually create it first:

        createdb goatcounter
        psql goatcounter < ./db/schema.pgsql
`

func cmdDb(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	defer func() { ready <- struct{}{} }()

	var (
		dbConnect = f.String("", "db")
	)
	err := f.Parse()
	if err != nil {
		return err
	}

	cmd := f.Shift()
	switch cmd {
	default:
		return errors.Errorf(`unknown command for "db": %q`, cmd)
	case "":
		return errors.New("need a subcommand: schema-sqlite, schema-pgsql, or test")

	case "schema-sqlite", "schema-pgsql":
		// TODO: Read from fs on dev
		d, err := goatcounter.DB.ReadFile("db/schema.gotxt")
		if err != nil {
			return err
		}
		driver := zdb.DriverSQLite
		if cmd == "schema-pgsql" {
			driver = zdb.DriverPostgreSQL
		}
		d, err = zdb.SchemaTemplate(driver, string(d))
		if err != nil {
			return err
		}
		fmt.Fprint(zli.Stdout, string(d))
		return nil
	case "test":
		if !dbConnect.Set() {
			return errors.New("must add -db flag")
		}
		db, err := zdb.Connect(zdb.ConnectOptions{Connect: dbConnect.String()})
		if err != nil {
			return err
		}
		defer db.Close()

		var i int
		err = db.Get(context.Background(), &i, `select 1 from version`)
		if err != nil {
			return fmt.Errorf("select 1 from version: %w", err)
		}
		fmt.Fprintln(zli.Stdout, "DB seems okay")
	}

	return nil
}

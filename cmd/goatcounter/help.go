// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const usageHelp = `
Show help; use "help commands" to dispay detailed help for a command, or "help
all" to display everything.
`

const helpDatabase = `
GoatCounter can use SQLite and PostgreSQL. All commands accept the -db flag to
customize the database connection string.

You can select a database engine by using "sqlite://[..]" for SQLite, or
"postgresql://[..]" (or "postgres://[..]") for PostgreSQL.

There are no plans to support other database engines such as MySQL/MariaDB.

SQLite

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

PostgreSQL

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

func help() (int, error) {
	if len(os.Args) == 2 {
		fmt.Fprint(stdout, usage[""])
		return 0, nil
	}

	if os.Args[2] == "all" {
		fmt.Fprint(stdout, usage[""], "\n")
		for _, h := range []string{
			"help", "version",
			"migrate", "create", "serve",
			"reindex", "monitor",
			"database",
		} {
			head := fmt.Sprintf("─── Help for %q ", h)
			fmt.Fprintf(stdout, "%s%s\n\n", head, strings.Repeat("─", 80-utf8.RuneCountInString(head)))
			fmt.Fprint(stdout, usage[h], "\n")
		}
		return 0, nil
	}

	t, ok := usage[os.Args[2]]
	if !ok {
		return 1, fmt.Errorf("no help topic for %q", os.Args[2])
	}
	fmt.Fprint(stdout, t)

	return 0, nil
}

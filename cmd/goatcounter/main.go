// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"           // PostgreSQL database driver.
	_ "github.com/mattn/go-sqlite3" // SQLite database driver.
	"zgo.at/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/db/migrate/gomig"
	"zgo.at/goatcounter/pack"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zstring"
	"zgo.at/zvalidate"
)

var version = "dev"

var (
	stdout = os.Stdout
	stderr = os.Stderr
	exit   = os.Exit
)

var usage = map[string]string{
	"":         usageTop,
	"help":     usageHelp,
	"serve":    usageServe,
	"create":   usageCreate,
	"migrate":  usageMigrate,
	"saas":     usageSaas,
	"reindex":  usageReindex,
	"monitor":  usageMonitor,
	"database": helpDatabase,
	"db":       helpDatabase,

	"version": `
Show version and build information. This is printed as key=value, separated by
semicolons.
`,
}

func init() {
	for k := range usage {
		usage[k] = strings.TrimSpace(usage[k]) + "\n"
	}
	errors.Package = "zgo.at/goatcounter"
}

const usageTop = `
Usage: goatcounter [command] [flags]

Commands:

  help         Show help; use "help <topic>" or "help all" for more details.
  version      Show version and build information and exit.
  migrate      Run database migrations.
  create       Create a new site and user.
  serve        Start HTTP server.

Advanced commands:

  reindex      Recreate the index tables (*_stats, *_count) from the hits.
  monitor      Monitor for pageviews.

Extra help topics:

  db           Documentation on the -db flag.

See "help <topic>" for more details for the command.
`

var CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

func main() {
	cfg.Version = version

	if len(os.Args) < 2 {
		fmt.Fprint(stdout, usage[""])
		exit(0)
		return
	}
	cmd := os.Args[1]
	CommandLine.SetOutput(stdout)
	CommandLine.Usage = func() { fmt.Fprint(stdout, "\n", usage[cmd], "\n") }

	var (
		code int
		err  error
	)
	switch cmd {
	default:
		printMsg(1, usage[""], "unknown command: %q", cmd)
		code = 1
	case "version":
		fmt.Fprintln(stdout, getVersion())
	case "help":
		code, err = help()
	case "migrate":
		code, err = migrate()
	case "create":
		code, err = create()
	case "serve":
		code, err = serve()
	case "saas":
		code, err = saas()
	case "reindex":
		code, err = reindex()
	case "monitor":
		code, err = monitor()
	}
	if err != nil {
		// code=1, the user did something wrong and print usage as well
		// code=2, some internal error, and print just that.
		if _, ok := err.(zvalidate.Validator); ok {
			printMsg(code, usage[cmd], err.Error())
		}
		printMsg(code, "", err.Error())
	}

	exit(code)
}

func printMsg(code int, usageText, msg string, args ...interface{}) {
	out := stdout
	if code > 0 {
		out = stderr
	}

	msg = strings.TrimSpace(msg)
	if msg != "" {
		fmt.Fprintf(out, msg+"\n", args...)
	}

	if usageText != "" {
		if msg != "" {
			fmt.Fprintf(out, "\n")
		}
		fmt.Fprint(out, usageText)
	}
}

func flagDB() *string    { return CommandLine.String("db", "sqlite://db/goatcounter.sqlite3", "") }
func flagDebug() *string { return CommandLine.String("debug", "", "") }

func connectDB(connect string, migrate []string, create bool) (*sqlx.DB, error) {
	cfg.PgSQL = strings.HasPrefix(connect, "postgresql://") || strings.HasPrefix(connect, "postgres://")

	opts := zdb.ConnectOptions{
		Connect: connect,
		Migrate: zdb.NewMigrate(nil, migrate,
			map[bool]map[string][]byte{true: pack.MigrationsPgSQL, false: pack.MigrationsSQLite}[cfg.PgSQL],
			map[bool]string{true: "db/migrate/pgsql", false: "db/migrate/sqlite"}[cfg.PgSQL]),
	}
	if create {
		opts.Schema = map[bool][]byte{true: pack.SchemaPgSQL, false: pack.SchemaSQLite}[cfg.PgSQL]
	}
	db, err := zdb.Connect(opts)
	if err != nil {
		return nil, err
	}

	if len(migrate) > 0 {
		err = runGoMigrations(db)
		return db, err
	}
	return db, nil
}

var goMigrations = map[string]func(zdb.DB) error{
	"2020-03-27-1-isbot": gomig.IsBot,
}

func runGoMigrations(db zdb.DB) error {
	var ran []string
	err := db.SelectContext(context.Background(), &ran,
		`select name from version order by name asc`)
	if err != nil {
		return errors.Errorf("runGoMigrations: %w", err)
	}

	ctx := zdb.With(context.Background(), db)

	for k, f := range goMigrations {
		if zstring.Contains(ran, k) {
			continue
		}
		zlog.Printf("running Go migration %q", k)

		err := zdb.TX(ctx, func(ctx context.Context, db zdb.DB) error {
			err := f(db)
			if err != nil {
				return errors.Errorf("runGoMigrations: running migration %q: %w", k, err)
			}

			_, err = db.ExecContext(context.Background(), `insert into version values ($1)`, k)
			if err != nil {
				return errors.Errorf("runGoMigrations: update version: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func getVersion() string {
	return fmt.Sprintf("version=%s; go=%s; GOOS=%s; GOARCH=%s; race=%t; cgo=%t",
		version, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		zruntime.Race, zruntime.CGO)
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"           // PostgreSQL database driver.
	_ "github.com/mattn/go-sqlite3" // SQLite database driver.
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/errorutil"
	"zgo.at/utils/runtimeutil"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

var version = "dev"

var usage = map[string]string{
	"":        usageTop,
	"help":    usageHelp,
	"serve":   usageServe,
	"create":  usageCreate,
	"migrate": usageMigrate,
	"saas":    usageSaas,
	"reindex": usageReindex,

	"version": `
Show version and build information. This is printed as key=value, separated by
semicolons.
`,
}

const usageTop = `
Usage: goatcounter [command] [flags]

Commands:

  help        Show help; use "help <command>" or "help all" for more details.
  version     Show version and build information and exit.
  migrate     Run database migrations.
  serve       Serve just existing domains. This is probably what you want if
              you're looking to self-host GoatCounter. Requires creating a site
              with "create" first.
  create      Create a new site and user.

Advanced commands:

  saas        Run a "SaaS" production server.
  reindex     Re-create the cached statistics (*_stats tables) from the hits.
              This is generally rarely needed and mostly a development tool.

See "help <command>" for more details for the command.
`

var CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

func main() {
	cfg.Version = version
	zlog.Config.StackFilter = errorutil.FilterPattern(
		errorutil.FilterTraceInclude, "zgo.at/goatcounter")

	if len(os.Args) < 2 {
		die(1, usage[""], "need a command")
	}
	cmd := os.Args[1]
	CommandLine.SetOutput(os.Stdout)
	CommandLine.Usage = func() { fmt.Print("\n", strings.TrimSpace(usage[cmd]), "\n") }

	var err error
	switch cmd {
	default:
		die(1, usage[""], "unknown command: %q", cmd)
	case "help":
		help()
	case "version":
		fmt.Println(getVersion())
	case "migrate":
		err = migrate()
	case "create":
		err = create()
	case "serve":
		err = serve()
	case "saas":
		err = saas()
	case "reindex":
		err = reindex()
	}
	if err != nil {
		if _, ok := err.(zvalidate.Validator); ok {
			die(1, usage[cmd], err.Error())
		}
		die(1, "", err.Error())
	}
}

func die(code int, usageText, msg string, args ...interface{}) {
	out := os.Stdout
	if code > 0 {
		out = os.Stderr
	}

	msg = strings.TrimSpace(msg)
	if msg != "" {
		fmt.Fprintf(out, msg+"\n", args...)
	}

	if usageText != "" {
		if msg != "" {
			fmt.Fprintf(out, "\n")
		}
		fmt.Fprintf(out, strings.TrimSpace(usageText)+"\n")
	}
	os.Exit(code)
}

func flagDB() *string    { return CommandLine.String("db", "sqlite://db/goatcounter.sqlite3", "") }
func flagDebug() *string { return CommandLine.String("debug", "", "") }

func connectDB(connect string, migrate []string) (*sqlx.DB, error) {
	cfg.PgSQL = strings.HasPrefix(connect, "postgresql://")
	return zdb.Connect(zdb.ConnectOptions{
		Connect: connect,
		Schema:  map[bool][]byte{true: pack.SchemaPgSQL, false: pack.SchemaSQLite}[cfg.PgSQL],
		Migrate: zdb.NewMigrate(nil, migrate,
			map[bool]map[string][]byte{true: pack.MigrationsPgSQL, false: pack.MigrationsSQLite}[cfg.PgSQL],
			map[bool]string{true: "db/migrate/pgsql", false: "db/migrate/sqlite"}[cfg.PgSQL]),
	})
}

func getVersion() string {
	return fmt.Sprintf("version=%s; go=%s; GOOS=%s; GOARCH=%s; race=%t; cgo=%t",
		version, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		runtimeutil.Race, runtimeutil.CGO)
}

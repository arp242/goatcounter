// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/getsentry/raven-go"
	"github.com/go-chi/chi"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"           // PostgreSQL database driver.
	_ "github.com/mattn/go-sqlite3" // SQLite database driver.
	"github.com/pkg/errors"
	"github.com/teamwork/reload"
	"github.com/teamwork/utils/errorutil"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zlog_sentry"

	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	dbinit "zgo.at/goatcounter/db"
	"zgo.at/goatcounter/handlers"
)

var (
	version         = "dev"
	requiredVersion = ""
)

func main() {
	cfg.Set()
	if cfg.Version == "" {
		cfg.Version = version
	}
	zmail.SMTP = cfg.SMTP
	fmt.Printf("Goatcounter version %s\n", version)
	cfg.Print()

	if cfg.Prod && cfg.SMTP == "" {
		panic("-prod enabled and -smtp not given")
	}

	defer zlog.ProfileCPU(cfg.CPUProfile)()

	zlog.Config.StackFilter = errorutil.FilterPattern(
		errorutil.FilterTraceInclude, "zgo.at/goatcounter")

	// Log to Sentry.
	if cfg.Sentry != "" {
		zlog.Config.Outputs = append(zlog.Config.Outputs, zlog_sentry.Report(cfg.Sentry, version))
	}

	// Reload on changes.
	if !cfg.Prod {
		go func() {
			err := reload.Do(zlog.Printf, reload.Dir("./tpl", zhttp.ReloadTpl))
			must(errors.Wrap(err, "reload.Do"))
		}()
	}

	// Connect to DB.
	var db *sqlx.DB
	if cfg.PgSQL {
		var err error
		db, err = sqlx.Connect("postgres", cfg.DBFile)
		must(errors.Wrap(err, "sqlx.Connect pgsql"))
		defer db.Close()

		// db.SetConnMaxLifetime()
		db.SetMaxIdleConns(25) // Default 2
		db.SetMaxOpenConns(25) // Default 0
	} else {
		exists := true
		if _, err := os.Stat(cfg.DBFile); os.IsNotExist(err) {
			zlog.Printf("database %q doesn't exist; loading new schema", cfg.DBFile)
			exists = false
		}
		var err error
		db, err = sqlx.Connect("sqlite3", cfg.DBFile)
		must(errors.Wrap(err, "sqlx.Connect sqlite"))
		defer db.Close()

		if !exists {
			db.MustExec(string(dbinit.Schema))
		}
	}

	if requiredVersion != "" {
		var version string
		must(db.Get(&version,
			`select name from version order by name desc limit 1`))
		if version != requiredVersion {
			zlog.Errorf("current DB version is %q, but need version %q; run migrations from ./db/migrate directory",
				version, requiredVersion)
		}
	}

	// Run background tasks.
	cron.Run(db)

	// Set up HTTP handler and servers.
	zhttp.Serve(&http.Server{Addr: cfg.Listen, Handler: zhttp.HostRoute(map[string]chi.Router{
		cfg.Domain:          zhttp.RedirectHost("//www." + cfg.Domain),
		"www." + cfg.Domain: handlers.NewWebsite(db),
		cfg.DomainStatic:    handlers.NewStatic("./public", cfg.Domain, cfg.Prod),
		"*." + cfg.Domain:   handlers.NewBackend(db),
	})}, func() {
		cron.Wait(db)
		raven.Wait()
		zlog.ProfileHeap(cfg.MemProfile)
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

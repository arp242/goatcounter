// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package main

import (
	"fmt"
	"net/http"
	"net/mail"

	"github.com/go-chi/chi"
	_ "github.com/lib/pq"           // PostgreSQL database driver.
	_ "github.com/mattn/go-sqlite3" // SQLite database driver.
	"github.com/pkg/errors"
	"github.com/teamwork/reload"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/errorutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
)

var version = "dev"

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

	// Setup logging.
	if cfg.Prod {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}
	zlog.Config.StackFilter = errorutil.FilterPattern(
		errorutil.FilterTraceInclude, "zgo.at/goatcounter")
	if cfg.EmailErrors != "" {
		zlog.Config.Outputs = append(zlog.Config.Outputs, func(l zlog.Log) {
			if l.Level != zlog.LevelErr {
				return
			}

			err := zmail.Send("GoatCounter Error",
				mail.Address{Address: "errors@zgo.at"},
				[]mail.Address{{Address: cfg.EmailErrors}},
				zlog.Config.Format(l))
			if err != nil {
				fmt.Println(err)
			}
		})
	}

	// Reload on changes.
	if !cfg.Prod {
		go func() {
			err := reload.Do(zlog.Printf, reload.Dir("./tpl", zhttp.ReloadTpl))
			must(errors.Wrap(err, "reload.Do"))
		}()
	}

	// Connect to DB.
	m := pack.MigrationsSQLite
	p := "db/migrate/sqlite"
	if cfg.PgSQL {
		m = pack.MigrationsPgSQL
		p = "db/migrate/pgsql"
	}
	// TODO: I'd like a switch to run these migrations:
	//   Show migration status and exit:             ./goatcounter -migrate
	//   Migrate all pending migrations and exit:    ./goatcounter -migrate all
	//   Migrate one and exit:                       ./goatcounter -migrate 2019-10-16-1-geoip
	//   Rollback last migration:                    ./goatcounter -rollback last
	//   Rollback specific migration:                ./goatcounter -rollback 2019-10-16-1-geoip
	db, err := zdb.Connect(cfg.DBFile, cfg.PgSQL, pack.SchemaSQLite, m, p)
	must(err)
	defer db.Close()

	// Run background tasks.
	cron.Run(db)
	acme.Run()

	// Set up HTTP handler and servers.
	d := zhttp.RemovePort(cfg.Domain)
	ds := zhttp.RemovePort(cfg.DomainStatic)
	zhttp.Serve(&http.Server{Addr: cfg.Listen, Handler: zhttp.HostRoute(map[string]chi.Router{
		d:                        zhttp.RedirectHost("//www." + cfg.Domain),
		"www." + d:               handlers.NewWebsite(db),
		ds:                       handlers.NewStatic("./public", cfg.Domain, cfg.Prod),
		"static.goatcounter.com": handlers.NewStatic("./public", cfg.Domain, cfg.Prod),
		"*":                      handlers.NewBackend(db),
	})}, func() {
		cron.Wait(db)
		acme.Wait()
		zlog.ProfileHeap(cfg.MemProfile)
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

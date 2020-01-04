// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"

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
	"zgo.at/utils/stringutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zstripe"
)

var version = "dev"

func main() {
	var migrate string
	flag.StringVar(&migrate, "migrate", "", "Run database migrations")
	cfg.Set()
	if cfg.Version == "" {
		cfg.Version = version
	}
	fmt.Printf("Goatcounter version %s\n", version)
	//cfg.Print()

	if cfg.Stripe != "" {
		for _, k := range stringutil.Fields(cfg.Stripe, ":") {
			switch {
			case strings.HasPrefix(k, "sk_"):
				zstripe.SecretKey = k
			case strings.HasPrefix(k, "pk_"):
				zstripe.PublicKey = k
			case strings.HasPrefix(k, "whsec_"):
				zstripe.SignSecret = k
			}
		}
	}
	if zstripe.SecretKey == "" || zstripe.SignSecret == "" || zstripe.PublicKey == "" {
		zstripe.SecretKey = ""
		zstripe.SignSecret = ""
		zstripe.PublicKey = ""
		zlog.Print("-stripe not given or doesn't contain all keys; billing disabled")
	}

	zmail.SMTP = cfg.SMTP

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
	db, err := zdb.Connect(zdb.ConnectOptions{
		Connect:    cfg.DBFile,
		PostgreSQL: cfg.PgSQL,
		Schema:     map[bool][]byte{true: pack.SchemaPgSQL, false: pack.SchemaSQLite}[cfg.PgSQL],
		Migrate: zdb.NewMigrate(nil, migrate,
			map[bool]map[string][]byte{true: pack.MigrationsPgSQL, false: pack.MigrationsSQLite}[cfg.PgSQL],
			map[bool]string{true: "db/migrate/pgsql", false: "db/migrate/sqlite"}[cfg.PgSQL]),
	})
	must(err)
	defer db.Close()

	// Don't continue if we just want to run migrations.
	if migrate != "" && migrate != "auto" {
		zlog.Print("migrations done")
		os.Exit(0)
	}

	// Run background tasks.
	cron.Run(db)
	acme.Run()

	// Set up HTTP handler and servers.
	domain := zhttp.RemovePort(cfg.Domain)
	hosts := map[string]chi.Router{
		domain:          zhttp.RedirectHost("//www." + cfg.Domain),
		"www." + domain: handlers.NewWebsite(db),
		"*":             handlers.NewBackend(db),
	}

	static := handlers.NewStatic("./public", cfg.Domain, cfg.Prod)
	for _, ds := range strings.Split(cfg.DomainStatic, ",") {
		hosts[zhttp.RemovePort(ds)] = static
	}

	zlog.Printf("listening on %q; prod: %t", cfg.Listen, cfg.Prod)
	zhttp.Serve(&http.Server{Addr: cfg.Listen, Handler: zhttp.HostRoute(hosts)}, func() {
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

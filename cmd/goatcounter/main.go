package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/getsentry/raven-go"
	"github.com/go-chi/chi"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/teamwork/reload"
	"github.com/teamwork/utils/errorutil"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zlog_sentry"

	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	dbinit "zgo.at/goatcounter/db"
	"zgo.at/goatcounter/handlers"
)

var version = "dev"

func main() {
	cfg.Set()
	if cfg.Version == "" {
		cfg.Version = version
	}
	fmt.Printf("Goatcounter version %s\n", version)
	cfg.Print()

	if cfg.Prod && cfg.SMTP == "" {
		panic("-prod enabled and -smtp not given")
	}

	if cfg.CPUProfile != "" {
		fp, err := os.Create(cfg.CPUProfile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(fp)
		defer pprof.StopCPUProfile()
	}

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
	var (
		db     *sqlx.DB
		exists = true
	)
	if cfg.PgSQL {
		var err error
		db, err = sqlx.Connect("postgres", "user=martin dbname=goatcounter sslmode=disable")
		must(errors.Wrap(err, "sqlx.Connect pgsql"))
		defer db.Close()

		db.MustExec(string(dbinit.Schema))

		// TODO(pg): check if exists
	} else {
		if _, err := os.Stat(cfg.DBFile); os.IsNotExist(err) {
			zlog.Printf("database %q doesn't exist; loading new schema", cfg.DBFile)
			exists = false
		}
		var err error
		db, err = sqlx.Connect("sqlite3", cfg.DBFile)
		must(errors.Wrap(err, "sqlx.Connect sqlite"))
		defer db.Close()
	}
	if !exists {
		db.MustExec(string(dbinit.Schema))
	}

	// Run background tasks.
	cron.Run(db)

	// Set up HTTP handler and servers.
	zhttp.Serve(&http.Server{Addr: cfg.Listen, Handler: zhttp.HostRoute(map[string]chi.Router{
		cfg.Domain:          zhttp.RedirectHost("//www." + cfg.Domain),
		"www." + cfg.Domain: handlers.NewSite(db),
		cfg.DomainStatic:    handlers.NewStatic("./public", cfg.Domain, cfg.Prod),
		"*." + cfg.Domain:   handlers.NewBackend(db),
	})}, func() {
		cron.Wait(db)
		raven.Wait()

		if cfg.MemProfile != "" {
			fp, err := os.Create(cfg.MemProfile)
			if err != nil {
				panic(err)
			}
			pprof.WriteHeapProfile(fp)
			fp.Close()
		}
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

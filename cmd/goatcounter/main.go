package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/getsentry/raven-go"
	"github.com/go-chi/chi"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/teamwork/reload"
	"github.com/teamwork/utils/errorutil"
	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter/cfg"
	dbinit "zgo.at/goatcounter/db"
	"zgo.at/goatcounter/handlers"
)

var version = "dev"

func main() {
	cfg.Set()
	fmt.Printf("Goatcounter version %s\n", version)
	cfg.Print()

	zlog.Config.StackFilter = errorutil.FilterPattern(
		errorutil.FilterTraceInclude, "zgo.at/goatcounter")

	// Log to Sentry.
	if cfg.Sentry != "" {
		err := raven.SetDSN(cfg.Sentry)
		must(errors.Wrap(err, "raven.SetDSN"))

		raven.SetRelease(version)

		zlog.Config.Output = func(l zlog.Log) {
			if l.Err == nil {
				fmt.Fprintln(os.Stdout, zlog.Config.Format(l))
				return
			}
			fmt.Fprintln(os.Stderr, zlog.Config.Format(l))

			data := make(map[string]string)
			for k, v := range l.Data {
				data[k] = fmt.Sprintf("%v", v)
			}
			raven.CaptureError(l.Err, data)
		}
	}

	// Reload on changes.
	if !cfg.Prod {
		go func() {
			err := reload.Do(zlog.Printf, reload.Dir("./tpl", zhttp.ReloadTpl))
			must(errors.Wrap(err, "reload.Do"))
		}()
	}

	// Connect to DB.
	// Connect to DB.
	exists := true
	if _, err := os.Stat(cfg.DBFile); os.IsNotExist(err) {
		zlog.Printf("database %q doesn't exist; loading new schema", cfg.DBFile)
		exists = false
	}
	db, err := sqlx.Connect("sqlite3", cfg.DBFile)
	must(errors.Wrap(err, "sqlx.Connect"))
	defer db.Close()

	if !exists {
		_, err := db.Exec(string(dbinit.Schema))
		must(errors.Wrap(err, "database schema init"))
	}

	// Set up HTTP handler and servers.
	zhttp.Serve(&http.Server{Addr: cfg.Listen, Handler: zhttp.HostRoute(map[string]chi.Router{
		cfg.Domain:        handlers.NewSite(db),
		cfg.DomainStatic:  handlers.NewStatic("./public", cfg.Domain, cfg.Prod),
		"*." + cfg.Domain: handlers.NewBackend(db),
	})}, raven.Wait)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

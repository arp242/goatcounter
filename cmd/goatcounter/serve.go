// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi"
	"github.com/teamwork/reload"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/ioutilx"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const usageServe = `
Serve existing domains, without the "SaaS" parts and billing. This is what you
want if you're looking to self-host.

Set up sites with the "create" command.

Flags:

  -db            Database connection string. Use "sqlite://<dbfile>" for SQLite,
                 or "postgres://<connect string>" for PostgreSQL
                 Default: sqlite://db/goatcounter.sqlite3

  -listen        Address to listen on. Default: localhost:8081

  -dev           Start in "dev mode".

  -static        Where to serve static files from.
                 Default: static.goatcounter.localhost:8081

  -smtp          SMTP server, as URL (e.g. "smtp://user:pass@server"). for
                 sending login emails and errors (if -errors is enabled).
                 Default is blank, meaning nothing is sent.

  -errors        What to do with errors; they're always printed to stderr.

                     mailto:addr     Email to this address; requires -smtp.

                 Default: not set.

  -debug         Modules to debug, comma-separated or 'all' for all modules.

  -automigrate   Automatically run all pending migrations on startup.

  -certdir       Directory to store ACME-generated certificates for custom
                 domains. Default: empty.

  -tls           Path to TLS certificate and key, colon-separated and in that
                 order. This will automatically redirect port 80 as well.
`

// TODO:
//
// - disable "admin" for site.id=1, and perhaps some other things as well?
// - Remove www links from footer.
// - Ports not reflected in domain links (login, script)
// - Settings → Code can be removed
// - Settings → Domain isn't filled in?
// - How to handle additional sites? Maybe just remove? Or add create flag?
// - Hide Delete account

func serve() error {
	dbConnect := flagDB()
	debug := flagDebug()

	var (
		automigrate, dev                  bool
		tls, listen, smtp, errors, static string
	)
	CommandLine.BoolVar(&automigrate, "automigrate", false, "")
	CommandLine.BoolVar(&dev, "dev", false, "")
	CommandLine.StringVar(&listen, "listen", "localhost:8081", "")
	CommandLine.StringVar(&smtp, "smtp", "", "")
	CommandLine.StringVar(&errors, "errors", "", "")
	CommandLine.StringVar(&cfg.CertDir, "certdir", "", "")
	CommandLine.StringVar(&tls, "tls", "", "")
	CommandLine.StringVar(&static, "static", "static.goatcounter.localhost:8081", "")
	CommandLine.Parse(os.Args[2:])

	zlog.Config.SetDebug(*debug)
	cfg.Prod = !dev
	cfg.SourceTree = ioutilx.Exists("./public/script.js") && ioutilx.Exists("./tpl/home.gohtml")
	cfg.DomainStatic = []string{static}
	zhttp.CookieSecure = !dev
	zmail.SMTP = smtp
	if !dev {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}

	v := zvalidate.New()
	if smtp == "" && !dev {
		v.Append("-smtp", "must be set if -dev is not enabled")
	}
	flagErrors(errors, &v)
	if v.HasErrors() {
		return v
	}

	// Reload on changes.
	if cfg.SourceTree {
		pack.Templates = nil
		pack.Public = nil

		go func() {
			err := reload.Do(zlog.Printf, reload.Dir("./tpl", zhttp.ReloadTpl))
			if err != nil {
				panic(fmt.Errorf("reload.Do: %v", err))
			}
		}()
	}

	// Connect to DB.
	db, err := connectDB(*dbConnect, map[bool][]string{true: []string{"all"}, false: nil}[automigrate])
	if err != nil {
		return err
	}
	defer db.Close()

	// Run background tasks.
	cron.Run(db)
	acme.Run()

	// Set up HTTP handler and servers.
	zhttp.InitTpl(pack.Templates)
	hosts := map[string]chi.Router{
		"*": handlers.NewBackend(db),
	}

	// Set up static host.
	// TODO: it would be better to serve static files from the same domain,
	// instead of requiring people to forward this one.
	// So instead of "static.example.com/script.js", something like
	// "stat.example.com/script.js" or "stat.example.com/public/script.js"
	// should also work.
	// This decreases the operational requirements to get this running a lot.
	hosts[zhttp.RemovePort(static)] = handlers.NewStatic("./public", "", !dev)

	cnames, err := lsSites(db)
	if err != nil {
		return err
	}
	zlog.Print(getVersion())
	zlog.Printf("serving %d sites on %q; dev=%t; sourceTree=%t",
		len(cnames), listen, dev, cfg.SourceTree)
	zlog.Printf("sites: %s", strings.Join(cnames, ", "))

	zhttp.Serve(&http.Server{Addr: listen, Handler: zhttp.HostRoute(hosts)}, tls, func() {
		cron.Wait(db)
		acme.Wait()
	})
	return nil
}

func lsSites(db zdb.DB) ([]string, error) {
	var sites goatcounter.Sites
	err := sites.List(zdb.With(context.Background(), db))
	if err != nil {
		return nil, err
	}

	var cnames []string
	for _, s := range sites {
		if s.Cname == nil {
			zlog.Errorf("cname is empty for site %d/%s", s.ID, s.Code)
			continue
		}
		cnames = append(cnames, *s.Cname)
	}

	return cnames, nil
}

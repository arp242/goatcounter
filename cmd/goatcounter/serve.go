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

  -static        Serve static files from a diffent domain, such as a CDN or
                 cookieless domain. Default: not set.

  -port          Port your site is publicly accessible on. Only needed if it's
                 not 80 or 443.

  -dev           Start in "dev mode".

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

func serve() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()

	var (
		automigrate, dev          bool
		tls, listen, smtp, errors string
	)
	CommandLine.BoolVar(&automigrate, "automigrate", false, "")
	CommandLine.BoolVar(&dev, "dev", false, "")
	CommandLine.StringVar(&listen, "listen", "localhost:8081", "")
	CommandLine.StringVar(&smtp, "smtp", "", "")
	CommandLine.StringVar(&errors, "errors", "", "")
	CommandLine.StringVar(&cfg.CertDir, "certdir", "", "")
	CommandLine.StringVar(&tls, "tls", "", "")
	CommandLine.StringVar(&cfg.Port, "port", "", "")
	CommandLine.StringVar(&cfg.DomainStatic, "static", "", "")
	CommandLine.Parse(os.Args[2:])

	zlog.Config.SetDebug(*debug)
	cfg.Prod = !dev
	zhttp.CookieSecure = !dev
	zmail.SMTP = smtp
	cfg.Serve = true
	if !dev {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}

	v := zvalidate.New()
	if smtp == "" && !dev {
		v.Append("-smtp", "must be set if -dev is not enabled")
	}
	flagErrors(errors, &v)

	if cfg.DomainStatic != "" {
		if p := strings.Index(cfg.DomainStatic, ":"); p > -1 {
			v.Domain("-domain", cfg.DomainStatic[:p])
		} else {
			v.Domain("-domain", cfg.DomainStatic)
		}
		cfg.URLStatic = "//" + cfg.DomainStatic
	}

	if v.HasErrors() {
		return 1, v
	}

	// Reload on changes.
	if !cfg.Prod {
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
		return 2, err
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
	if cfg.DomainStatic != "" {
		hosts[zhttp.RemovePort(cfg.DomainStatic)] = handlers.NewStatic("./public", cfg.Domain, !dev)
	}

	cnames, err := lsSites(db)
	if err != nil {
		return 2, err
	}
	zlog.Print(getVersion())
	zlog.Printf("serving %q on %q; dev=%t", cfg.Domain, listen, dev)
	zlog.Printf("%d sites: %s", len(cnames), strings.Join(cnames, ", "))

	zhttp.Serve(&http.Server{Addr: listen, Handler: zhttp.HostRoute(hosts)}, tls, func() {
		cron.Wait(db)
		acme.Wait()
	})

	return 0, nil
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

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"context"
	"net/http"
	"strings"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const usageServe = `
Serve existing domains, without the "SaaS" parts and billing. This is what you
want if you're looking to self-host.

Set up sites with the "create" command.

Flags:

  -static        Serve static files from a diffent domain, such as a CDN or
                 cookieless domain. Default: not set.

  -port          Port your site is publicly accessible on. Only needed if it's
                 not 80 or 443.
` + serveAndSaasFlags

const serveAndSaasFlags = `
  -db            Database connection string. Use "sqlite://<dbfile>" for SQLite,
                 or "postgres://<connect string>" for PostgreSQL
                 Default: sqlite://db/goatcounter.sqlite3

  -listen        Address to listen on. Default: localhost:8081

  -dev           Start in "dev mode".

  -tls           Serve over tls. This is a comma-separated list with any of:

                   none              Don't serve any TLS.
                   path/to/file.pem  TLS certificate and keyfile, in one file.
                   acme              Create TLS certificates with ACME, this can
                                     optionally followed by a : and a cache
                                     directory name (default: acme-secrets).
                   tls               Accept TLS connections on -listen.
                   rdr               Redirect port 80.

                 Examples:

                   acme                       Create ACME certs but serve HTTP,
                                              useful when serving behind proxy
                                              which can use the certs.

                   acme:/home/gc/.acme        As above, but with custom cache dir.

                   ./example.com.pem,tls,rdr  Always use the certificate in the
                                              file, serve over TLS, and redirect
                                              port 80.

                 Default: "acme,tls,rdr" for serve, "acme" for saas, and blank
                 when -dev is given.

  -smtp          SMTP server, as URL (e.g. "smtp://user:pass@server"). for
                 sending login emails and errors (if -errors is enabled).
                 Default is blank, meaning nothing is sent.

  -errors        What to do with errors; they're always printed to stderr.

                     mailto:addr     Email to this address; requires -smtp.

                 Default: not set.

  -debug         Modules to debug, comma-separated or 'all' for all modules.

  -automigrate   Automatically run all pending migrations on startup.
`

func serve() (int, error) {
	CommandLine.StringVar(&cfg.Port, "port", "", "")
	CommandLine.StringVar(&cfg.DomainStatic, "static", "", "")
	dbConnect, dev, automigrate, listen, smtp, tls, errors := flagServeAndSaas()

	cfg.Serve = true
	if tls == "" {
		tls = map[bool]string{true: "none", false: "acme,tls,rdr"}[dev]
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
		setupReload()
	}

	db, err := connectDB(dbConnect, map[bool][]string{true: []string{"all"}, false: nil}[automigrate], true)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	zhttp.InitTpl(pack.Templates)
	tlsc, acmeh, listenTLS := acme.Setup(db, tls)
	defer setupCron(db)

	// Set up HTTP handler and servers.
	hosts := map[string]http.Handler{
		"*": handlers.NewBackend(db, acmeh),
	}
	if cfg.DomainStatic != "" {
		hosts[zhttp.RemovePort(cfg.DomainStatic)] = handlers.NewStatic("./public", cfg.Domain, !dev)
	}

	cnames, err := lsSites(db)
	if err != nil {
		return 2, err
	}
	zlog.Print(getVersion())
	zlog.Printf("serving %d sites on %q; dev=%t:", len(cnames), listen, dev)
	zlog.Printf("  %s", strings.Join(cnames, ", "))
	zhttp.Serve(listenTLS, &http.Server{
		Addr:      listen,
		Handler:   zhttp.HostRoute(hosts),
		TLSConfig: tlsc,
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

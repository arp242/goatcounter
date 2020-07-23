// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const usageServe = `
Start a HTTP server to serve one or more GoatCounter installations.

Set up sites with the "create" command; you don't need to restart for changes to
take effect.

Static files and templates are compiled in the binary and aren't needed to run
GoatCounter. But they're loaded from the filesystem if GoatCounter is started
with -dev.

Flags:

  -db          Database connection: "sqlite://<file>" or "postgres://<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite://db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -listen      Address to listen on. Default: "*:443", or "localhost:8081" with
               -dev. See "goatcounter help listen" for detailed documentation.

  -tls         Serve over tls. This is a comma-separated list with any of:

                 none                   Don't serve any TLS
                 tls                    Accept TLS connections on -listen
                 path/to/file.pem       TLS certificate and keyfile, in one file
                 acme[:cache]           Create TLS certificates with ACME
                 rdr                    Redirect port 80 to the -listen port

               Default: "acme,tls,rdr", or "none" when -dev is given.
               See "goatcounter help listen" for more detailed documentation.

  -port        Port your site is publicly accessible on. Only needed if it's
               not 80 or 443.

  -automigrate Automatically run all pending migrations on startup.

  -smtp        SMTP server, as URL (e.g. "smtp://user:pass@server").

               A special value of "stdout" means no emails will be sent and
               emails will be printed to stdout only. This is the default.

               If this is blank emails will be sent without using a relay; this
               should work fine, but deliverability will usually be worse (i.e.
               it will be more likely to end up in the spam box). This usually
               requires rDNS properly set up, and GoatCounter will *not* retry
               on errors. Using stdout, a local smtp relay, or a mailtrap.io box
               is probably better unless you really know what you're doing.

  -email-from  From: address in emails. Default: <user>@<hostname>

  -errors      What to do with errors; they're always printed to stderr.

                 mailto:to_addr[,from_addr]  Email to this address; the
                                             from_addr is optional and sets the
                                             From: address. The default is to
                                             use the same as the to_addr.
               Default: not set.

  -static      Serve static files from a different domain, such as a CDN or
               cookieless domain. Default: not set.

  -dev         Start in "dev mode".

  -debug       Modules to debug, comma-separated or 'all' for all modules.

Environment:

  TMPDIR       Directory for temporary files; only used to store CSV exports at
               the moment. On Windows it will use the first non-empty value of
               %TMP%, %TEMP%, and %USERPROFILE%.
`

func serve() (int, error) {
	v := zvalidate.New()

	CommandLine.StringVar(&cfg.Port, "port", "", "")
	CommandLine.StringVar(&cfg.DomainStatic, "static", "", "")
	dbConnect, dev, automigrate, listen, tls, from, err := flagServeAndSaas(&v)
	if err != nil {
		return 1, err
	}

	cfg.Serve = true
	if tls == "" {
		tls = map[bool]string{true: "none", false: "acme,tls,rdr"}[dev]
	}

	if cfg.DomainStatic != "" {
		if p := strings.Index(cfg.DomainStatic, ":"); p > -1 {
			v.Domain("-static", cfg.DomainStatic[:p])
		} else {
			v.Domain("-static", cfg.DomainStatic)
		}
		cfg.URLStatic = "//" + cfg.DomainStatic
		cfg.DomainCount = cfg.DomainStatic
	}

	if cfg.Port != "" {
		cfg.Port = ":" + cfg.Port
	}

	flagFrom(from, &v)
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

	err = goatcounter.Memstore.Init(db)
	if err != nil {
		return 0, err
	}

	cronWait := setupCron(db)
	defer func() {
		defer cronWait()
		defer bgrun.WaitAndLog()
		defer goatcounter.Memstore.StoreSessions(db)
		zlog.Print("Waiting for background tasks to finish…")
	}()

	// Set up HTTP handler and servers.
	hosts := map[string]http.Handler{
		"*": handlers.NewBackend(db, acmeh),
	}
	if cfg.DomainStatic != "" {
		// May not be needed, but just in case the DomainStatic isn't an
		// external CDN.
		hosts[zhttp.RemovePort(cfg.DomainStatic)] = handlers.NewStatic(chi.NewRouter(), "./public", !dev)
	}

	cnames, err := lsSites(db)
	if err != nil {
		return 2, err
	}

	zlog.Module("main").Debug(getVersion())
	zhttp.Serve(listenTLS, &http.Server{
		Addr:      listen,
		Handler:   zhttp.HostRoute(hosts),
		TLSConfig: tlsc,

		// Set some reasonably high timeouts which should never be reached.
		// Note that handlers have a 5-second timeout set in handlers/mw.go
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, func() {
		banner()
		zlog.Printf("ready; serving %d sites on %q; dev=%t; sites: %s",
			len(cnames), listen, dev, strings.Join(cnames, ", "))

		if len(cnames) == 0 {
			zlog.Errorf("No sites yet; create a new site with:\n    goatcounter create -domain [..] -email [..]")
		}
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

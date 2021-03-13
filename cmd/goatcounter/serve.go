// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/teamwork/reload"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/handlers"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ztpl"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/znet"
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
                 path/to/file.pem       TLS certificate and keyfile, in one file
                 acme[:cache]           Create TLS certificates with ACME
                 rdr                    Redirect port 80 to the -listen port

               Default: "acme,rdr", or "none" when -dev is given.
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

  -geodb       Path to mmdb GeoIP database; can be either the City or Country
               version, but regional information is only recorded with the City
               version.

               This parameter is optional; GoatCounter comes with a Countries
               version built-in; you only need this if you want to use a
               newer/different version, or if you want to record regions.

  -dev         Start in "dev mode".

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

Environment:

  TMPDIR       Directory for temporary files; only used to store CSV exports at
               the moment. On Windows it will use the first non-empty value of
               %TMP%, %TEMP%, and %USERPROFILE%.
`

func cmdServe(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	v := zvalidate.New()

	var (
		port         = f.String("", "port").Pointer()
		domainStatic = f.String("", "static").Pointer()
	)
	dbConnect, dev, automigrate, listen, flagTLS, from, err := flagsServe(f, &v)
	if err != nil {
		return err
	}

	return func(port, domainStatic string) error {
		if flagTLS == "" {
			flagTLS = map[bool]string{true: "none", false: "acme,rdr"}[dev]
		}

		var domainCount, urlStatic string
		if domainStatic != "" {
			if p := strings.Index(domainStatic, ":"); p > -1 {
				v.Domain("-static", domainStatic[:p])
			} else {
				v.Domain("-static", domainStatic)
			}
			urlStatic = "//" + domainStatic
			domainCount = domainStatic
		}

		//from := flagFrom(from, "cfg.Domain", &v)
		from := flagFrom(from, "", &v)
		if v.HasErrors() {
			return v
		}

		db, ctx, tlsc, acmeh, listenTLS, err := setupServe(dbConnect, dev, flagTLS, automigrate)
		if err != nil {
			return err
		}

		c := goatcounter.Config(ctx)
		c.EmailFrom = from
		if port != "" {
			c.Port = ":" + port
		}
		c.DomainStatic = domainStatic
		c.Serve = true
		c.Dev = dev
		c.URLStatic = urlStatic
		c.DomainCount = domainCount

		// Set up HTTP handler and servers.
		hosts := map[string]http.Handler{
			"*": handlers.NewBackend(db, acmeh, dev, c.GoatcounterCom, c.DomainStatic),
		}
		if domainStatic != "" {
			// May not be needed, but just in case the DomainStatic isn't an
			// external CDN.
			hosts[znet.RemovePort(domainStatic)] = handlers.NewStatic(chi.NewRouter(), dev)
		}

		cnames, err := lsSites(ctx)
		if err != nil {
			return err
		}

		return doServe(ctx, db, listen, listenTLS, tlsc, hosts, stop, func() {
			banner()
			zlog.Printf("ready; serving %d sites on %q; dev=%t; sites: %s",
				len(cnames), listen, dev, strings.Join(cnames, ", "))
			if len(cnames) == 0 {
				zlog.Errorf("No sites yet; create a new site with:\n    goatcounter create -domain [..] -email [..]")
			}
			ready <- struct{}{}
		})
	}(*port, *domainStatic)
}

func doServe(ctx context.Context, db zdb.DB,
	listen string, listenTLS uint8, tlsc *tls.Config, hosts map[string]http.Handler,
	stop chan struct{}, start func(),
) error {

	var sig = make(chan os.Signal, 1)
	zlog.Module("startup").Debug(getVersion())
	ch, err := zhttp.Serve(listenTLS, stop, &http.Server{
		Addr:        listen,
		Handler:     zhttp.HostRoute(hosts),
		TLSConfig:   tlsc,
		BaseContext: func(net.Listener) context.Context { return ctx },
	})
	if err != nil {
		return err
	}

	<-ch // Server is set up
	start()

	<-ch // Shutdown
	go func() {
		signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, os.Interrupt /*SIGINT*/)
		<-sig
		zli.Colorln("One more to kill…", zli.Bold)
		<-sig
		zli.Colorln("Force killing", zli.Bold)
		os.Exit(99) // TODO: zli.Exit?
	}()

	bgrun.Run("shutdown", func() {
		err := cron.PersistAndStat(goatcounter.CopyContextValues(ctx))
		if err != nil {
			zlog.Error(err)
		}
		goatcounter.Memstore.StoreSessions(db)
	})
	zlog.Print("Waiting for background tasks to finish; send HUP, TERM, or INT twice to force kill (may lose data!)")
	time.Sleep(10 * time.Millisecond)
	bgrun.WaitProgressAndLog()

	db.Close()
	return nil
}

func flagsServe(f zli.Flags, v *zvalidate.Validator) (string, bool, bool, string, string, string, error) {
	var (
		dbConnect   = f.String("sqlite://db/goatcounter.sqlite3", "db").Pointer()
		debug       = f.String("", "debug").Pointer()
		dev         = f.Bool(false, "dev").Pointer()
		automigrate = f.Bool(false, "automigrate").Pointer()
		listen      = f.String(":443", "listen").Pointer()
		smtp        = f.String(blackmail.ConnectWriter, "smtp").Pointer()
		flagTLS     = f.String("", "tls").Pointer()
		errors      = f.String("", "errors").Pointer()
		from        = f.String("", "email-from").Pointer()
		geodb       = f.String("", "geodb").Pointer()
	)
	err := f.Parse()

	zlog.Config.SetDebug(*debug)
	zhttp.LogUnknownFields = *dev
	zhttp.CookieSecure = !*dev
	if *flagTLS == "none" {
		zhttp.CookieSecure = false
	}

	if !*dev {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}

	flagErrors(*errors, v)

	if *smtp != blackmail.ConnectDirect && *smtp != blackmail.ConnectWriter {
		v.URLLocal("-smtp", *smtp)
	}
	blackmail.DefaultMailer = blackmail.NewMailer(*smtp)

	goatcounter.InitGeoDB(*geodb)

	return *dbConnect, *dev, *automigrate, *listen, *flagTLS, *from, err
}

func setupServe(dbConnect string, dev bool, flagTLS string, automigrate bool) (zdb.DB, context.Context, *tls.Config, http.HandlerFunc, uint8, error) {
	if dev {
		setupReload()
	}

	db, ctx, err := connectDB(dbConnect, map[bool][]string{true: {"all"}, false: {"pending"}}[automigrate], true, dev)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}

	fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "tpl", dev)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	ztpl.Init(fsys)

	tlsc, acmeh, listenTLS := acme.Setup(db, flagTLS, dev)

	err = goatcounter.Memstore.Init(db)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}

	cron.RunBackground(goatcounter.CopyContextValues(ctx))
	return db, ctx, tlsc, acmeh, listenTLS, nil
}

func setupReload() {
	if _, err := os.Stat("./tpl"); os.IsNotExist(err) {
		return
	}

	go func() {
		err := reload.Do(zlog.Module("startup").Debugf, reload.Dir("./tpl", func() { ztpl.Reload("./tpl") }))
		if err != nil {
			panic(errors.Errorf("reload.Do: %v", err))
		}
	}()
}

func flagErrors(errors string, v *zvalidate.Validator) {
	switch {
	default:
		v.Append("-errors", "invalid value")
	case errors == "":
		// Do nothing.
	case strings.HasPrefix(errors, "mailto:"):
		errors = errors[7:]
		s := strings.Split(errors, ",")
		from := s[0]
		to := s[0]
		if len(s) > 1 {
			to = s[1]
		}

		v.Email("-errors", from)
		v.Email("-errors", to)
		zlog.Config.Outputs = append(zlog.Config.Outputs, func(l zlog.Log) {
			if l.Level != zlog.LevelErr {
				return
			}

			bgrun.Run("email:error", func() {
				err := blackmail.Send("GoatCounter Error",
					blackmail.From("", from),
					blackmail.To(to),
					blackmail.BodyText([]byte(zlog.Config.Format(l))))
				if err != nil {
					// Just output to stderr I guess, can't really do much more if
					// zlog fails.
					fmt.Fprintf(zli.Stderr, "emailerrors: %s\n", err)
				}
			})
		})
	}
}

func flagFrom(from, domain string, v *zvalidate.Validator) string {
	if from == "" {
		if domain != "" { // saas only.
			from = "support@" + znet.RemovePort(domain)
		} else {
			u, err := user.Current()
			if err != nil {
				panic("cannot get user for -email-from parameter")
			}
			h, err := os.Hostname()
			if err != nil {
				panic("cannot get hostname for -email-from parameter")
			}
			from = fmt.Sprintf("%s@%s", u.Username, h)
		}

	}

	// TODO
	// if zmail.SMTP != "stdout" {
	// 	v.Email("-email-from", from, fmt.Sprintf("%q is not a valid email address", from))
	// }
	return from
}

func lsSites(ctx context.Context) ([]string, error) {
	var sites goatcounter.Sites
	err := sites.UnscopedList(goatcounter.CopyContextValues(ctx))
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

func banner() {
	fmt.Fprint(zli.Stdout, `
┏━━━━━━━━━━━━━━━━━━━━━ Thank you for using GoatCounter! ━━━━━━━━━━━━━━━━━━━━━━┓
┃                                                                             ┃
┃ Great you're choosing to self-host GoatCounter! I'd just like to put a      ┃
┃ reminder here that I work on this full-time; it's not a side-project.       ┃
┃ Please consider making a financial contribution according to your means if  ┃
┃ this is useful for you to ensure the long-term viability. Thank you :-)     ┃
┃                                                                             ┃
┃                   https://www.goatcounter.com/contribute                    ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛

`)
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"

	"github.com/teamwork/reload"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/stringutil"
	"zgo.at/zhttp"
	"zgo.at/zhttp/zmail"
	"zgo.at/zlog"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

// saas
const usageSaas = `
Run as a "SaaS" service; this will run, a public-facing website on
www.[domanin], a static file server on [staticdomain], and a backend UI on
[code].domain. Users are expected to register on www.[domain].

Static files and templates are compiled in the binary and aren't needed to run
GoatCounter. But they're loaded from the filesystem if GoatCounter is started
with -dev.

Flags:

  -domain        Base domain with port followed by comma and the static domain.
                 Default: goatcounter.localhost:8081, static.goatcounter.localhost:8081

  -plan          Plan for new installations; default: personal.

  -stripe        Stripe keys; needed for billing. It needs the secret,
                 publishable, and webhook (sk_*, pk_*, whsec_*) keys as
                 colon-separated, in any order. Billing will be disabled if left
                 blank.
` + serveAndSaasFlags

func saas() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()

	var (
		automigrate, dev                                bool
		tls, listen, smtp, errors, stripe, domain, plan string
	)
	CommandLine.BoolVar(&automigrate, "automigrate", false, "")
	CommandLine.BoolVar(&dev, "dev", false, "")
	CommandLine.StringVar(&domain, "domain", "goatcounter.localhost:8081,static.goatcounter.localhost:8081", "")
	CommandLine.StringVar(&listen, "listen", "localhost:8081", "")
	CommandLine.StringVar(&smtp, "smtp", "", "")
	CommandLine.StringVar(&errors, "errors", "", "")
	CommandLine.StringVar(&stripe, "stripe", "", "")
	CommandLine.StringVar(&plan, "plan", goatcounter.PlanPersonal, "")
	CommandLine.StringVar(&tls, "tls", "", "")
	CommandLine.Parse(os.Args[2:])

	zlog.Config.SetDebug(*debug)
	cfg.Prod = !dev
	cfg.Plan = plan
	cfg.Saas = true
	zhttp.LogUnknownFields = dev
	zhttp.CookieSecure = !dev
	zmail.SMTP = smtp
	if !dev {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}

	v := zvalidate.New()
	v.Include("-plan", plan, goatcounter.Plans)
	//v.URL("-smtp", smtp) // TODO smtp://localhost fails (1 domain label)
	// TODO: validate tls
	if smtp == "" && !dev {
		v.Append("-smtp", "must be set if -dev is not enabled")
	}
	flagErrors(errors, &v)
	flagStripe(stripe, &v)
	flagDomain(domain, &v)
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
	db, err := connectDB(*dbConnect, map[bool][]string{true: {"all"}, false: nil}[automigrate], true)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	// Run background tasks.
	cron.Run(db)
	defer cron.Wait(db)

	// Set up HTTP handler and servers.
	zhttp.InitTpl(pack.Templates)
	tlsc, acmeh, listenTLS := handlers.SetupTLS(db, tls)

	d := zhttp.RemovePort(cfg.Domain)
	hosts := map[string]http.Handler{
		zhttp.RemovePort(cfg.DomainStatic): handlers.NewStatic("./public", cfg.Domain, !dev),
		d:                                  zhttp.RedirectHost("//www." + cfg.Domain),
		"www." + d:                         handlers.NewWebsite(db),
		"*":                                handlers.NewBackend(db, acmeh),
	}

	zlog.Print(getVersion())
	zlog.Printf("serving %q on %q; dev=%t", cfg.Domain, listen, dev)
	zhttp.Serve(listenTLS, &http.Server{
		Addr:      listen,
		Handler:   zhttp.HostRoute(hosts),
		TLSConfig: tlsc,
	})
	return 0, nil
}

func flagErrors(errors string, v *zvalidate.Validator) {
	switch {
	default:
		v.Append("-errors", "invalid value")
	case errors == "":
		// Do nothing.
	case strings.HasPrefix(errors, "mailto:"):
		errors = errors[7:]
		v.Email("-errors", errors)
		zlog.Config.Outputs = append(zlog.Config.Outputs, func(l zlog.Log) {
			if l.Level != zlog.LevelErr {
				return
			}

			err := zmail.Send("GoatCounter Error",
				mail.Address{Address: "errors@zgo.at"},
				[]mail.Address{{Address: errors}},
				zlog.Config.Format(l))
			if err != nil {
				// Just output to stderr I guess, can't really do much more if
				// zlog fails.
				fmt.Fprintf(stderr, "emailerrors: %s\n", err)
			}
		})
	}
}

func flagStripe(stripe string, v *zvalidate.Validator) {
	if stripe == "" {
		zlog.Print("-stripe not given; billing disabled")
		return
	}

	for _, k := range stringutil.Fields(stripe, ":") {
		switch {
		case strings.HasPrefix(k, "sk_"):
			zstripe.SecretKey = k
		case strings.HasPrefix(k, "pk_"):
			zstripe.PublicKey = k
		case strings.HasPrefix(k, "whsec_"):
			zstripe.SignSecret = k
		}
	}
	if zstripe.SecretKey == "" {
		v.Append("-stripe", "missing secret key (sk_)")
	}
	if zstripe.PublicKey == "" {
		v.Append("-stripe", "missing public key (pk_)")
	}
	if zstripe.SignSecret == "" {
		v.Append("-stripe", "missing signing secret (whsec_)")
	}
}

func flagDomain(domain string, v *zvalidate.Validator) {
	l := strings.Split(domain, ",")

	switch len(l) {
	case 0:
		v.Append("-domain", "cannot be blank")
	case 1, 2:
		for i, d := range l {
			d = strings.TrimSpace(d)
			if p := strings.Index(d, ":"); p > -1 {
				v.Domain("-domain", d[:p])
			} else {
				v.Domain("-domain", d)
			}

			if i == 0 {
				cfg.Domain = d
			} else {
				cfg.DomainStatic = d
				cfg.URLStatic = "//" + d
			}
		}
	default:
		v.Append("-domain", "too many domains")
	}

}

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
	"time"

	"github.com/teamwork/reload"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/stringutil"
	"zgo.at/zdb"
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

func flagServeAndSaas(v *zvalidate.Validator) (string, bool, bool, string, string, string) {
	dbConnect := flagDB()
	debug := flagDebug()

	var dev bool
	CommandLine.BoolVar(&dev, "dev", false, "")
	automigrate := CommandLine.Bool("automigrate", false, "")
	listen := CommandLine.String("listen", "localhost:8081", "")
	smtp := CommandLine.String("smtp", "stdout", "")
	tls := CommandLine.String("tls", "", "")
	errors := CommandLine.String("errors", "", "")

	CommandLine.Parse(os.Args[2:])

	zlog.Config.SetDebug(*debug)
	cfg.Prod = !dev
	zhttp.LogUnknownFields = dev
	zhttp.CookieSecure = !dev
	zmail.SMTP = *smtp
	if !dev {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}

	//v.URL("-smtp", smtp) // TODO smtp://localhost fails (1 domain label)

	return *dbConnect, dev, *automigrate, *listen, *tls, *errors
}

func setupReload() {
	pack.Templates = nil
	pack.Public = nil
	go func() {
		err := reload.Do(zlog.Printf, reload.Dir("./tpl", zhttp.ReloadTpl))
		if err != nil {
			panic(fmt.Errorf("reload.Do: %v", err))
		}
	}()
}

func setupCron(db zdb.DB) func() {
	cron.RunBackground(db)
	go func() {
		defer zlog.Recover()
		time.Sleep(3 * time.Second)
		cron.RunOnce(db)
	}()
	return func() { cron.Wait(db) }
}

func saas() (int, error) {
	v := zvalidate.New()

	var stripe, domain, plan string
	CommandLine.StringVar(&domain, "domain", "goatcounter.localhost:8081,static.goatcounter.localhost:8081", "")
	CommandLine.StringVar(&stripe, "stripe", "", "")
	CommandLine.StringVar(&plan, "plan", goatcounter.PlanPersonal, "")
	dbConnect, dev, automigrate, listen, tls, errors := flagServeAndSaas(&v)

	cfg.Saas = true
	cfg.Plan = plan
	if tls == "" {
		tls = map[bool]string{true: "none", false: "acme"}[dev]
	}

	v.Include("-plan", plan, goatcounter.Plans)
	flagErrors(errors, &v)
	flagStripe(stripe, &v)
	flagDomain(domain, &v)
	if v.HasErrors() {
		return 1, v
	}

	if !cfg.Prod {
		setupReload()
	}

	db, err := connectDB(dbConnect, map[bool][]string{true: {"all"}, false: nil}[automigrate], true)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	zhttp.InitTpl(pack.Templates)
	tlsc, acmeh, listenTLS := acme.Setup(db, tls)
	defer setupCron(db)()

	// Set up HTTP handler and servers.
	d := zhttp.RemovePort(cfg.Domain)
	hosts := map[string]http.Handler{
		zhttp.RemovePort(cfg.DomainStatic): handlers.NewStatic("./public", cfg.Domain, !dev),
		d:                                  zhttp.RedirectHost("//www." + cfg.Domain),
		"www." + d:                         handlers.NewWebsite(db),
		"*":                                handlers.NewBackend(db, acmeh),
	}

	zlog.Print(getVersion())
	zlog.Printf("serving %q on %q; dev=%t", cfg.Domain, listen, dev)
	banner()
	zhttp.Serve(listenTLS, &http.Server{
		Addr:      listen,
		Handler:   zhttp.HostRoute(hosts),
		TLSConfig: tlsc,
	})
	return 0, nil
}

func banner() {
	fmt.Print(`
┏━━━━━━━━━━━━━━━━━━━━━ Thank you for using GoatCounter! ━━━━━━━━━━━━━━━━━━━━━━┓
┃                                                                             ┃
┃ Great you're choosing to self-host GoatCounter! I'd just like to put a      ┃
┃ reminder here that I work on this full-time; it's not a side-project.       ┃
┃ Please consider making a financial contribution according to your means if  ┃
┃ this is useful for you to ensure the long-term viability. Thank you :-)     ┃
┃                                                                             ┃
┃                     https://www.goatcounter/contribute                      ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
`)
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

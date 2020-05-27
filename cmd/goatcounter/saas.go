// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/teamwork/reload"
	"zgo.at/blackmail"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/acme"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/handlers"
	"zgo.at/goatcounter/pack"
	"zgo.at/utils/stringutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

// saas
const usageSaas = `
NOTE: running your own SaaS is currently undocumented, non-trivial, and has
certain assumptions that will not be true in your case. You do not want to run
this; for now it can only run run goatcounter.com

Run as a "SaaS" service; this will run, a public-facing website on
www.[domanin], a static file server on [staticdomain], and a backend UI on
[code].domain. Users are expected to register on www.[domain].

Static files and templates are compiled in the binary and aren't needed to run
GoatCounter. But they're loaded from the filesystem if GoatCounter is started
with -dev.

Flags:

  -domain        Base domain with port followed by comma and the static domain,
                 optionally followed by the doimain to serve count.js
                 Default: goatcounter.localhost:8081, static.goatcounter.localhost:8081
                 Example: -domain goatcounter.com,static.zgo.at,gc.zgo.at

  -plan          Plan for new installations; default: personal.

  -stripe        Stripe keys; needed for billing. It needs the secret,
                 publishable, and webhook (sk_*, pk_*, whsec_*) keys as
                 colon-separated, in any order.
` + serveAndSaasFlags

func flagServeAndSaas(v *zvalidate.Validator) (string, bool, bool, string, string, string, error) {
	dbConnect := flagDB()
	debug := flagDebug()

	var dev bool
	CommandLine.BoolVar(&dev, "dev", false, "")
	automigrate := CommandLine.Bool("automigrate", false, "")
	listen := CommandLine.String("listen", "localhost:8081", "")
	smtp := CommandLine.String("smtp", blackmail.ConnectWriter, "")
	tls := CommandLine.String("tls", "", "")
	errors := CommandLine.String("errors", "", "")
	from := CommandLine.String("email-from", "", "")

	err := CommandLine.Parse(os.Args[2:])
	zlog.Config.SetDebug(*debug)
	cfg.Prod = !dev
	zhttp.LogUnknownFields = dev
	zhttp.CookieSecure = !dev
	if *tls == "none" {
		zhttp.CookieSecure = false
	}

	if !dev {
		zlog.Config.FmtTime = "Jan _2 15:04:05 "
	}

	flagErrors(*errors, v)

	if *smtp != blackmail.ConnectDirect && *smtp != blackmail.ConnectWriter {
		v.URL("-smtp", *smtp)
	}
	blackmail.DefaultMailer = blackmail.NewMailer(*smtp)

	return *dbConnect, dev, *automigrate, *listen, *tls, *from, err
}

func setupReload() {
	if _, err := os.Stat("./tpl"); os.IsNotExist(err) {
		return
	}

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
	dbConnect, dev, automigrate, listen, tls, from, err := flagServeAndSaas(&v)
	if err != nil {
		return 1, err
	}

	cfg.GoatcounterCom = true
	cfg.Plan = plan
	if tls == "" {
		tls = map[bool]string{true: "none", false: "acme"}[dev]
	}

	v.Include("-plan", plan, goatcounter.Plans)
	flagStripe(stripe, &v)
	flagDomain(domain, &v)
	flagFrom(from, &v)
	if cfg.Prod && cfg.Domain != "goatcounter.com" {
		v.Append("saas", "can only run on goatcounter.com")
	}

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

			go func() {
				defer zlog.Recover()
				err := blackmail.Send("GoatCounter Error",
					blackmail.From("", from),
					blackmail.To(to),
					blackmail.Bodyf(zlog.Config.Format(l)))
				if err != nil {
					// Just output to stderr I guess, can't really do much more if
					// zlog fails.
					fmt.Fprintf(stderr, "emailerrors: %s\n", err)
				}
			}()
		})
	}
}

func flagFrom(from string, v *zvalidate.Validator) {
	if from == "" {
		if cfg.Domain != "" { // saas only.
			from = "support@" + zhttp.RemovePort(cfg.Domain)
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

	cfg.EmailFrom = from

	// TODO
	// if zmail.SMTP != "stdout" {
	// 	v.Email("-email-from", from, fmt.Sprintf("%q is not a valid email address", from))
	// }
}

func flagStripe(stripe string, v *zvalidate.Validator) {
	if stripe == "" {
		v.Required("-stripe", stripe)
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
	default:
		v.Append("-domain", "too many domains")
	case 0:
		v.Append("-domain", "cannot be blank")
	case 1:
		v.Append("-domain", "must have static domain")
	case 2, 3:
		for i, d := range l {
			d = strings.TrimSpace(d)
			if p := strings.Index(d, ":"); p > -1 {
				v.Domain("-domain", d[:p])
			} else {
				v.Domain("-domain", d)
			}

			switch i {
			case 0:
				cfg.Domain = d
			case 1:
				cfg.DomainStatic = d
				cfg.DomainCount = d
				cfg.URLStatic = "//" + d
			case 2:
				cfg.DomainCount = d
			}
		}
	}

}

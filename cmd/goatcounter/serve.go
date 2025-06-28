package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/teamwork/reload"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/text/language"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/acme"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/handlers"
	"zgo.at/goatcounter/v2/pkg/bgrun"
	"zgo.at/goatcounter/v2/pkg/email_log"
	"zgo.at/goatcounter/v2/pkg/geo"
	"zgo.at/goatcounter/v2/pkg/geo/geoip2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/z18n"
	"zgo.at/zhttp"
	"zgo.at/zli"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/zio"
	"zgo.at/zstd/znet"
	"zgo.at/zstd/zruntime"
	"zgo.at/ztpl"
	"zgo.at/zvalidate"
)

const usageServe = `
Start a HTTP server to serve one or more GoatCounter installations.

Set up sites with the "create" command; you don't need to restart for changes
to take effect.

Static files and templates are compiled in the binary and aren't needed to run
GoatCounter. But they're loaded from the filesystem if GoatCounter is started
with -dev.

Environment:

  All of the flags take the defaults from $GOATCOUNTER_«FLAG», where «FLAG» is
  the flag name. The commandline flag will override the environment variable.

  For example:

    GOATCOUNTER_LISTEN=:80
    GOATCOUNTER_STORE_EVERY=60
    GOATCOUNTER_AUTOMIGRATE=

  Additional environment variables:


    TMPDIR              Directory for temporary files; only used to store CSV
                        exports at the moment. On Windows it will use the first
                        non-empty value of %TMP%, %TEMP%, and %USERPROFILE%.
    GOATCOUNTER_TMPDIR  Alternative way to set TMPDIR; takes precedence over
                        TMPDIR. Mainly intended for cases where TMPDIR can't be
                        used (e.g. when the capability bit is set on Linux).

Flags:

  -db          Database connection: "sqlite+<file>" or "postgres+<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite+./db/goatcounter.sqlite3 if that database file exists, or
               sqlite+./goatcounter-data/db.sqlite3 if it doesn't.

  -dbconn      Set maximum number of connections, as max_open,max_idle

               There is no maximum if max_open is -1, and idle connections are
               not retained if max_idle is -1 The default is 16,4.

  -listen      Address to listen on. Default: "*:8080". See "goatcounter help
               listen" for detailed documentation.

  -tls         Serve over tls. This is a comma-separated list with any of:

                 http                   Don't serve any TLS (default)
                 path/to/file.pem       TLS certificate and key, in one file
                 acme[:cache]           Create TLS certificates with ACME
                 rdr                    Redirect port 80 to the -listen port

               See "goatcounter help listen" for detailed documentation.

  -public-port Port your site is publicly accessible on. Only needed if it's
               not 80 or 443.

  -base-path   Path under which GoatCounter is available. Usually GoatCounter
               runs on its own domain or subdomain ("stats.example.com"), but
               in some cases it's useful to run GoatCounter under a path
               ("example.com/stats"), in which case you'll need to set this to
               "/stats".

  -automigrate Automatically run all pending migrations on startup.

  -smtp        SMTP relay server, as URL (e.g. "smtp://user:pass@server:port").
               TLS connections are supported via smtps:// or STARTTLS.

               When the debug query parameter is present ("smtp://...?debug=1")
               all client/server traffic will be written to stderr.

               A special value of "stdout" or "stderr" will print emails to
               stdout or stderr without actually sending them The default is
               "stdout".

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

               GoatCounter will automatically use the first .mmdb file in
               ./goatcounter-data, if any exists. GoatCounter comes with a
               Countries version built-in, and will use that if this flag isn't
               given and there is no file in ./goatcounter-data. You only need
               this if you want to use a newer/different version, or if you
               want to record regions.

               This can also be a MaxMind account ID and license key, in which
               case GoatCounter will automatically download a Cities database
               from MaxMind and update it every week. The format for this is:

                   maxmind:account_id:license[:path]

               :path may be omitted and defaults to goatcounter-data/auto.mmdb.

               For example:

                   -geodb 123456:abcdef
                   -geodb 123456:abcdef:/home/goatcounter/cities.mmd

               Updates are only done on restarts.

  -ratelimit   Set rate limits for various actions; the syntax is
               "name:num-requests/seconds"; multiple values are separated by
               a comma. The defaults are:

                   count:4/1            4 requests / second
                   api:4/1              4 requests / seconds
                   api-count:60/120    60 requests / 2 minutes
                   export:1/3600        1 requests / hour
                   login:20/60         20 requests / minute

               If one of the names is omitted it will fall back to the default
               value; for example "-ratelimit export:3/3600,api:100/1" will use
               the default for "count", "login", etc.

  -api-max     Maximum number of items /api/ endpoints will return. Set to 0
               for the defaults (200 for paths, 100 for everything else), or <0
               for no limit.

  -store-every How often to persist pageviews to the database, in seconds.
               Higher values will give better performance, but it will take a
               bit longer for pageviews to show. The default is 10 seconds.

  -dev         Start in "dev mode".

  -json        Output logs as JSON instead of aligned text.

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.
`

func cmdServe(f zli.Flags, ready chan<- struct{}, stop chan struct{}, saas bool) error {
	var (
		port         = f.Int(0, "public-port", "port") // TODO(depr): -port is for compat with <2.0
		basePath     = f.String("", "base-path")
		domainStatic = f.String("", "static")
		dbConnect    = f.String(defaultDB(), "db")
		dbConn       = f.String("16,4", "dbconn")
		debugFlag    = f.StringList(nil, "debug")
		dev          = f.Bool(false, "dev")
		automigrate  = f.Bool(false, "automigrate")
		listen       = f.String(":8080", "listen")
		smtp         = f.String("stdout", "smtp")
		flagTLS      = f.String("http", "tls")
		errorsFlag   = f.String("", "errors")
		from         = f.String("", "email-from")
		geodbFlag    = f.String("", "geodb")
		ratelimit    = f.String("", "ratelimit")
		apiMax       = f.Int(0, "api-max")
		storeEvery   = f.Int(10, "store-every")
		json         = f.Bool(false, "json")
		_            = f.Bool(false, "websocket") // TODO(depr): no-op for compat with <2.7

		// For saas
		domain = f.String("goatcounter.localhost:8081,static.goatcounter.localhost:8081", "domain")
	)
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil {
		return err
	}

	v := zvalidate.New()

	setupLog(dev.Bool(), json.Bool(), debugFlag.StringsSplit(","))

	if dev.Bool() {
		zhttp.DefaultDecoder = zhttp.NewDecoder(true, false) // Log unknown fields
		if err := setupReload(); err != nil {
			return err
		}
	}
	if flagTLS.String() == "" {
		*flagTLS.Pointer() = map[bool]string{true: "http", false: "acme"}[dev.Bool()]
	}

	flagErrors(&v, errorsFlag.String())
	mailer := flagEmail(&v, smtp.String())
	geodb := setupGeo(&v, geodbFlag.String())
	ratelimits := setupRatelimits(&v, ratelimit.String())
	*from.Pointer() = flagFrom(&v, saas, from.String(), domain.String())
	domainCount, urlStatic := setupDomains(&v, saas, dev.Bool(), domain.Pointer(),
		domainStatic.Pointer(), basePath.Pointer())

	v.Range("-store-every", int64(storeEvery.Int()), 1, 0)
	cron.SetPersistInterval(time.Duration(storeEvery.Int()) * time.Second)

	if v.HasErrors() {
		return v
	}

	db, ctx, err := connectDB(dbConnect.String(), dbConn.String(),
		map[bool][]string{true: {"all"}, false: {"pending"}}[automigrate.Bool()],
		true, dev.Bool())
	if err != nil {
		return err
	}
	defer db.Close()

	ctx = z18n.With(ctx, z18n.NewBundle(language.English).Locale("en"))
	ctx = geo.With(ctx, geodb)
	ctx = blackmail.With(ctx, mailer)

	if err := setupTpl(ctx, dev.Bool()); err != nil {
		return err
	}

	tlsc, acmeh, listenTLS := acme.Setup(db, flagTLS.String(), dev.Bool())

	zhttp.ErrPage = handlers.ErrPage
	zhttp.CookieSameSiteHelper = handlers.SameSite

	if err := goatcounter.Memstore.Init(db); err != nil {
		return err
	}

	cron.Start(goatcounter.CopyContextValues(ctx))

	c := goatcounter.Config(ctx)
	c.GoatcounterCom = saas
	c.Domain = domain.String()
	c.DomainStatic = domainStatic.String()
	c.DomainCount = domainCount
	c.URLStatic = urlStatic
	c.Dev = dev.Bool()
	c.BasePath = basePath.String()
	c.EmailFrom = from.String()

	if port.Int() > 0 {
		c.Port = fmt.Sprintf(":%d", port.Int())
	}

	timeout := 60
	if saas {
		timeout = 15
	}

	// Set up HTTP handler and servers.
	hosts := map[string]http.Handler{
		"*": handlers.NewBackend(db, acmeh, dev.Bool(), c.GoatcounterCom, c.DomainStatic, c.BasePath, timeout, apiMax.Int(), ratelimits),
	}
	if saas {
		d := znet.RemovePort(domain.String())
		hosts[d] = zhttp.RedirectHost("https://www." + domain.String())
		hosts["www."+d] = handlers.NewWebsite(db, dev.Bool())
		if dev.Bool() {
			hosts[znet.RemovePort(domainStatic.String())] = handlers.NewStatic(chi.NewRouter(), dev.Bool(), true, c.BasePath)
		}
	}
	if domainStatic.String() != "" {
		// May not be needed, but just in case the DomainStatic isn't an
		// external CDN.
		hosts[znet.RemovePort(domainStatic.String())] = handlers.NewStatic(chi.NewRouter(), dev.Bool(), false, c.BasePath)
	}

	var cnames []string
	if !saas {
		cnames, err = lsSites(ctx)
		if err != nil {
			return err
		}
	}

	ch, err := zhttp.Serve(listenTLS, stop, &http.Server{
		Addr: listen.String(),
		// TODO: h2c no longer needed? https://github.com/golang/go/issues/72039
		Handler:     h2c.NewHandler(zhttp.HostRoute(hosts), &http2.Server{}),
		TLSConfig:   tlsc,
		BaseContext: func(net.Listener) context.Context { return ctx },
	})
	if err != nil {
		return err
	}

	<-ch // Server is set up

	extra := []any{"num_sites", len(cnames), "sites", cnames}
	if saas {
		extra = []any{"domain", domain}
	}
	log.Module("startup").Info(ctx, "GoatCounter ready",
		startupAttr(geodb, listen.String(), dev.Bool(), extra...)...)

	if !saas && len(cnames) == 0 {
		dbFlag := ""
		if dbConnect.String() != defaultDB() {
			dbFlag = `-db="` + strings.ReplaceAll(dbConnect.String(), `"`, `\"`) + `" `
		}
		// Adjust command for Docker or Podman
		cmd := "goatcounter"
		if _, err := os.Stat("/.dockerenv"); err == nil && os.Getenv("HOSTNAME") != "" {
			cmd = "docker exec -it " + os.Getenv("HOSTNAME") + " goatcounter"
		}
		if _, err := os.Stat("/run/.containerenv"); err == nil && os.Getenv("HOSTNAME") != "" {
			cmd = "podman exec -it " + os.Getenv("HOSTNAME") + " goatcounter"
		}
		log.Warnf(ctx, "No sites yet; access the web interface or use the CLI to create one:\n"+
			"    %s db %screate site -vhost=.. -user.email=..", cmd, dbFlag)
	}

	ready <- struct{}{}

	<-ch // Shutdown

	sig := make(chan os.Signal, 1)
	go func() {
		signal.Notify(sig, syscall.SIGHUP, syscall.SIGTERM, os.Interrupt /*SIGINT*/)
		<-sig
		zli.Colorln("One more to kill…", zli.Bold)
		<-sig
		zli.Colorln("Force killing", zli.Bold)
		os.Exit(99) // TODO: zli.Exit?
	}()

	bgrun.RunFunction("shutdown", func() {
		err := cron.TaskPersistAndStat()
		if err != nil {
			log.Error(ctx, err)
		}
		goatcounter.Memstore.StoreSessions(db)
	})

	time.Sleep(200 * time.Millisecond) // Only show message if it doesn't exit in 200ms.

	first := true
	for r := bgrun.Running(); len(r) > 0; r = bgrun.Running() {
		if first {
			log.Info(ctx, "Waiting for background tasks; send HUP, TERM, or INT twice to force kill")
			first = false
		}
		time.Sleep(100 * time.Millisecond)

		zli.Erase()
		fmt.Fprintf(zli.Stdout, "\r%d tasks: ", len(r))
		for i, t := range r {
			if i > 0 {
				fmt.Fprint(zli.Stdout, ", ")
			}
			fmt.Fprintf(zli.Stdout, "%s (%s)", t.Task, time.Since(t.Started).Round(time.Second))
		}
	}
	fmt.Fprintln(zli.Stdout)
	return nil
}

func defaultDB() string {
	if _, err := os.Stat("./db/goatcounter.sqlite3"); err == nil {
		return "sqlite+./db/goatcounter.sqlite3"
	}
	return "sqlite+./goatcounter-data/db.sqlite3"
}

func setupReload() error {
	if !zio.Exists("db/migrate") || !zio.Exists("tpl") || !zio.Exists("public") {
		return errors.New("-dev flag was given but this doesn't seem like a GoatCounter source directory")
	}
	if _, err := exec.LookPath("git"); err == nil {
		rev := ""
		b, ok := debug.ReadBuildInfo()
		if ok {
			for _, s := range b.Settings {
				if s.Key == "vcs.revision" {
					rev = s.Value
				}
			}
		}
		if rev != "" {
			have, err := exec.Command("git", "log", "-n1", "--pretty=format:%H").CombinedOutput()
			if err == nil {
				if h := strings.TrimSpace(string(have)); rev != h {
					log.Errorf(context.Background(),
						"goatcounter was built from revision %s but source directory has revision %s",
						rev[:7], h[:7])
				}
			}
		}
	}

	if _, err := os.Stat("./tpl"); os.IsNotExist(err) {
		return nil
	}

	go func() {
		l := func(s string, a ...any) { log.Module("startup").Infof(context.Background(), s, a...) }
		err := reload.Do(l, reload.Dir("./tpl", func() {
			if err := ztpl.Reload("./tpl"); err != nil {
				log.Error(context.Background(), err)
			}
		}))
		if err != nil {
			log.Errorf(context.Background(), "reload.Do: %v", err)
		}
	}()
	return nil
}

func flagErrors(v *zvalidate.Validator, errors string) {
	switch {
	default:
		v.Append("-errors", "invalid value")
	case errors == "":
		// Do nothing.
	case strings.HasPrefix(errors, "mailto:"):
		to, from, _ := strings.Cut(errors[7:], ",")
		if from == "" {
			from = to
		}

		v.Email("-errors", from)
		v.Email("-errors", to)
		slog.SetDefault(slog.New(log.NewChain(
			slog.Default().Handler(),
			email_log.New(slog.LevelWarn, from, to),
		)))
	}
}

func flagEmail(v *zvalidate.Validator, smtp string) blackmail.Mailer {
	var (
		m   blackmail.Mailer
		err error
	)
	switch {
	case strings.ToLower(smtp) == "stdout":
		m = blackmail.NewWriter(os.Stdout)
	case strings.ToLower(smtp) == "stderr":
		m = blackmail.NewWriter(os.Stderr)
	default:
		v.URLLocal("-smtp", smtp)

		var opt blackmail.RelayOptions
		u, _ := url.Parse(smtp)
		if u.Query().Has("debug") {
			opt.Debug = os.Stderr
		}
		m, err = blackmail.NewRelay(smtp, &opt)
	}
	if err != nil {
		v.Append("-smtp", fmt.Sprintf("setting up mailer: %s", err))
	}

	return m
}

func setupGeo(v *zvalidate.Validator, geodbFlag string) *geoip2.Reader {
	if geodbFlag == "" {
		ls, _ := os.ReadDir("goatcounter-data")
		for _, f := range ls {
			if strings.HasSuffix(f.Name(), ".mmdb") {
				geodbFlag = "goatcounter-data/" + f.Name()
				break
			}
		}
	}
	geodb, err := geo.Open(geodbFlag)
	if err != nil {
		v.Append("-geodb", fmt.Sprintf("loading GeoIP database: %s", err))
	}
	return geodb
}

func setupRatelimits(v *zvalidate.Validator, ratelimit string) handlers.Ratelimits {
	h := handlers.NewRatelimits()
	if ratelimit != "" {
		for _, r := range strings.Split(ratelimit, ",") {
			name, spec, _ := strings.Cut(r, ":")
			reqs, secs, _ := strings.Cut(spec, "/")

			v2 := zvalidate.New()
			v2.Required("-ratelimit.name", name)
			v2.Required("-ratelimit.requests", reqs)
			v2.Required("-ratelimit.seconds", secs)
			nn := v2.Include("-ratelimit.name", name, []string{"count", "api", "api-count", "export", "login"})
			name = nn.(string)
			r := v2.Integer("-ratelimit.requests", reqs)
			s := v2.Integer("-ratelimit.seconds", secs)
			if v2.HasErrors() {
				v.Merge(v2)
			}
			h.Set(name, int(r), s)
		}
	}
	return h
}

func flagFrom(v *zvalidate.Validator, saas bool, from, domain string) string {
	if from == "" {
		if saas && domain != "" { // saas only.
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
			log.Errorf(ctx, "cname is empty for site %d/%s", s.ID, s.Code)
			continue
		}
		cnames = append(cnames, *s.Cname)
	}

	return cnames, nil
}

func setupDomains(v *zvalidate.Validator, saas, dev bool, domain, domainStatic, basePath *string) (string, string) {
	*basePath = strings.Trim(*basePath, "/")
	if *basePath != "" {
		*basePath = "/" + *basePath
	}
	zhttp.BasePath = *basePath

	var domainCount, urlStatic string
	if saas {
		*domain, *domainStatic, domainCount, urlStatic = flagDomain(v, *domain)
		if !dev && *domain != "goatcounter.com" {
			v.Append("saas", "can only run on goatcounter.com")
		}
	} else {
		if *domainStatic != "" {
			if p := strings.Index(*domainStatic, ":"); p > -1 {
				v.Domain("-static", (*domainStatic)[:p])
			} else {
				v.Domain("-static", *domainStatic)
			}
			urlStatic = "//" + *domainStatic
			domainCount = *domainStatic
		} else {
			urlStatic = *basePath
		}
	}
	return domainCount, urlStatic
}

func flagDomain(v *zvalidate.Validator, domain string) (string, string, string, string) {
	l := strings.Split(domain, ",")

	var (
		rDomain      string
		domainStatic string
		domainCount  string
		urlStatic    string
	)
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
				rDomain = d
			case 1:
				domainStatic = d
				domainCount = d
				urlStatic = "//" + d
			case 2:
				domainCount = d
			}
		}
	}
	return rDomain, domainStatic, domainCount, urlStatic
}

func setupTpl(ctx context.Context, dev bool) error {
	fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "tpl", dev)
	if err != nil {
		return err
	}
	err = ztpl.Init(fsys)
	if err != nil {
		if !dev {
			return err
		}
		log.Error(ctx, err)
	}
	return nil
}

func startupAttr(geodb *geoip2.Reader, listen string, dev bool, attr ...any) []any {
	md := geodb.DB().Metadata
	return append(attr,
		"listen", listen,
		"dev", dev,
		slog.Group("version",
			"version", goatcounter.Version,
			"go", runtime.Version(),
			"GOOS", runtime.GOOS,
			"GOARCH", runtime.GOARCH,
			"CGO", zruntime.CGO,
			"race", zruntime.Race,
		),
		slog.Group("geoip",
			"path", geodb.DB().Path,
			"build", time.Unix(int64(md.BuildEpoch), 0).UTC().Format("2006-01-02 15:04:05"),
			"type", md.DatabaseType,
			"description", md.Description["en"],
			"nodes", md.NodeCount,
		),
	)
}

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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
	"github.com/oschwald/maxminddb-golang"
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
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/slog_align"
	"zgo.at/z18n"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zli"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/zio"
	"zgo.at/zstd/znet"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zstring"
	"zgo.at/ztpl"
	"zgo.at/zvalidate"
)

const usageServe = `
Start a HTTP server to serve one or more GoatCounter installations.

Set up sites with the "create" command; you don't need to restart for changes to
take effect.

Static files and templates are compiled in the binary and aren't needed to run
GoatCounter. But they're loaded from the filesystem if GoatCounter is started
with -dev.

Environment:

  All of the flags take the defaults from $GOATCOUNTER_«FLAG», where «FLAG» is
  the flag name. The commandline flag will override the environment variable.

  For example:

    GOATCOUNTER_LISTEN=:80
    GOATCOUNTER_STORE_EVERY=60
    GOATCOUNTER_WEBSOCKET=

  Additional environment variables:

    TMPDIR       Directory for temporary files; only used to store CSV exports
                 at the moment. On Windows it will use the first non-empty value
                 of %TMP%, %TEMP%, and %USERPROFILE%.

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
                 path/to/file.pem       TLS certificate and keyfile, in one file
                 acme[:cache]           Create TLS certificates with ACME
                 rdr                    Redirect port 80 to the -listen port

               See "goatcounter help listen" for detailed documentation.

  -public-port Port your site is publicly accessible on. Only needed if it's
               not 80 or 443.

  -base-path   Path under which GoatCounter is available. Usually GoatCounter
               runs on its own domain or subdomain ("stats.example.com"), but in
               some cases it's useful to run GoatCounter under a path
               ("example.com/stats"), in which case you'll need to set this to
               "/stats".

  -automigrate Automatically run all pending migrations on startup.

  -smtp        SMTP relay server, as URL (e.g. "smtp://user:pass@server").

               A special value of "stdout" will print emails to stdout without
               actually sending them.  This is the default.

               If this is an empty string (-smtp='') emails will be sent without
               using a relay. This implementation is very simple and
               deliverability will usually be bad (i.e. it will end up in the
               spam box, or just be outright rejected). This usually requires
               rDNS properly set up, and GoatCounter will *not* retry on errors.
               Using a local smtp relay is almost always better unless you
               really know what you're doing.

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
               ./goatcounter-data, if any exists.

               GoatCounter comes with a Countries version built-in, and will use
               that if this flag isn't given and there is no file in
               ./goatcounter-data. You only need this if you want to use a
               newer/different version, or if you want to record regions.

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

  -api-max     Maximum number of items /api/ endpoints will return. Set to 0 for
               the defaults (200 for paths, 100 for everything else), or <0 for
               no limit.

  -websocket   Use a websocket to send data. The advantage of this is that the
               perceived performance is quite a bit better, especially with a
               lot of data, since things can be loaded "lazily". The downside is
               that it doesn't work out-of-the-box with a proxy setup (e.g.
               nginx, Apache, Varnish, etc.) and requires special configuration,
               which is why it's disabled by default.

  -store-every How often to persist pageviews to the database, in seconds.
               Higher values will give better performance, but it will take a
               bit longer for pageviews to show. The default is 10 seconds.

  -dev         Start in "dev mode".

  -json        Output logs as JSON instead of aligned text.

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.
`

func cmdServe(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	v := zvalidate.New()

	var (
		// TODO(depr): -port is for compat with <2.0
		port         = f.Int(0, "public-port", "port").Pointer()
		basePath     = f.String("/", "base-path").Pointer()
		domainStatic = f.String("", "static").Pointer()
	)
	dbConnect, dbConn, dev, automigrate, listen, flagTLS, from, websocket, apiMax, ratelimits, geomd, err := flagsServe(f, &v)
	if err != nil {
		return err
	}

	return func(port int, basePath, domainStatic string) error {
		basePath = strings.Trim(basePath, "/")
		if basePath != "" {
			basePath = "/" + basePath
		}
		zhttp.BasePath = basePath

		var domainCount, urlStatic string
		if domainStatic != "" {
			if p := strings.Index(domainStatic, ":"); p > -1 {
				v.Domain("-static", domainStatic[:p])
			} else {
				v.Domain("-static", domainStatic)
			}
			urlStatic = "//" + domainStatic
			domainCount = domainStatic
		} else {
			urlStatic = basePath
		}

		//from := flagFrom(from, "cfg.Domain", &v)
		from := flagFrom(from, "", &v)
		if v.HasErrors() {
			return v
		}

		db, ctx, tlsc, acmeh, listenTLS, err := setupServe(dbConnect, dbConn, dev, flagTLS, automigrate)
		if err != nil {
			return err
		}

		c := goatcounter.Config(ctx)
		c.EmailFrom = from
		if port > 0 {
			c.Port = fmt.Sprintf(":%d", port)
		}
		c.DomainStatic = domainStatic
		c.Dev = dev
		c.URLStatic = urlStatic
		c.BasePath = basePath
		c.DomainCount = domainCount
		c.Websocket = websocket

		// Set up HTTP handler and servers.
		hosts := map[string]http.Handler{
			"*": handlers.NewBackend(db, acmeh, dev, c.GoatcounterCom, websocket, c.DomainStatic, c.BasePath, 60, apiMax, ratelimits),
		}
		if domainStatic != "" {
			// May not be needed, but just in case the DomainStatic isn't an
			// external CDN.
			hosts[znet.RemovePort(domainStatic)] = handlers.NewStatic(chi.NewRouter(), dev, false, c.BasePath)
		}

		cnames, err := lsSites(ctx)
		if err != nil {
			return err
		}

		return doServe(ctx, db, listen, listenTLS, tlsc, hosts, stop, func() {
			log.Module("startup").Info(ctx, "GoatCounter ready", startupAttr(geomd, listen, dev,
				"num_sites", len(cnames),
				"sites", cnames,
			)...)

			if len(cnames) == 0 {
				dbFlag := ""
				if dbConnect != defaultDB() {
					dbFlag = `-db="` + strings.ReplaceAll(dbConnect, `"`, `\"`) + `" `
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
		})
	}(*port, *basePath, *domainStatic)
}

func doServe(ctx context.Context, db zdb.DB,
	listen string, listenTLS uint8, tlsc *tls.Config, hosts map[string]http.Handler,
	stop chan struct{}, start func(),
) error {

	var sig = make(chan os.Signal, 1)
	ch, err := zhttp.Serve(listenTLS, stop, &http.Server{
		Addr:        listen,
		Handler:     h2c.NewHandler(zhttp.HostRoute(hosts), &http2.Server{}),
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
	db.Close()
	return nil
}

func defaultDB() string {
	if _, err := os.Stat("./db/goatcounter.sqlite3"); err == nil {
		return "sqlite+./db/goatcounter.sqlite3"
	}
	return "sqlite+./goatcounter-data/db.sqlite3"
}

type geometa struct {
	path string
	md   maxminddb.Metadata
}

func flagsServe(f zli.Flags, v *zvalidate.Validator) (string, string, bool, bool, string, string, string, bool, int, handlers.Ratelimits, geometa, error) {
	var (
		dbConnect   = f.String(defaultDB(), "db").Pointer()
		dbConn      = f.String("16,4", "dbconn").Pointer()
		debug       = f.StringList(nil, "debug")
		dev         = f.Bool(false, "dev").Pointer()
		automigrate = f.Bool(false, "automigrate").Pointer()
		listen      = f.String(":8080", "listen").Pointer()
		smtp        = f.String(blackmail.ConnectWriter, "smtp").Pointer()
		flagTLS     = f.String("http", "tls").Pointer()
		errors      = f.String("", "errors").Pointer()
		from        = f.String("", "email-from").Pointer()
		geodb       = f.String("", "geodb").Pointer()
		ratelimit   = f.String("", "ratelimit").Pointer()
		apiMax      = f.Int(0, "api-max").Pointer()
		storeEvery  = f.Int(10, "store-every").Pointer()
		websocket   = f.Bool(false, "websocket").Pointer()
		json        = f.Bool(false, "json").Pointer()
	)
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil {
		return "", "", false, false, "", "", "", false, 0, handlers.Ratelimits{}, geometa{}, err
	}

	setupLog(*dev, *json, debug.StringsSplit(","))
	if *dev {
		zhttp.DefaultDecoder = zhttp.NewDecoder(true, false)
	}

	flagErrors(*errors, v)

	if *smtp != blackmail.ConnectDirect && *smtp != blackmail.ConnectWriter {
		v.URLLocal("-smtp", *smtp)
	}
	blackmail.DefaultMailer = blackmail.NewMailer(*smtp)

	v.Range("-store-every", int64(*storeEvery), 1, 0)
	cron.SetPersistInterval(time.Duration(*storeEvery) * time.Second)

	if *geodb == "" {
		ls, _ := os.ReadDir("goatcounter-data")
		for _, f := range ls {
			if strings.HasSuffix(f.Name(), ".mmdb") {
				*geodb = "goatcounter-data/" + f.Name()
				break
			}
		}
	}
	geomd, err := goatcounter.InitGeoDB(*geodb)
	if err != nil {
		return "", "", false, false, "", "", "", false, 0, handlers.Ratelimits{}, geometa{},
			fmt.Errorf("loading GeoIP database: %w", err)
	}
	if *geodb == "" {
		*geodb = "(builtin)"
	}
	md := geometa{*geodb, geomd}

	ratelimits := handlers.NewRatelimits()
	if *ratelimit != "" {
		for _, r := range strings.Split(*ratelimit, ",") {
			name, spec, _ := strings.Cut(r, ":")
			reqs, secs, _ := strings.Cut(spec, "/")

			v := zvalidate.New()
			v.Required("name", name)
			v.Required("requests", reqs)
			v.Required("seconds", secs)
			nn := v.Include("name", name, []string{"count", "api", "api-count", "export", "login"})
			name = nn.(string)
			r := v.Integer("requests", reqs)
			s := v.Integer("seconds", secs)
			if v.HasErrors() {
				return *dbConnect, *dbConn, *dev, *automigrate, *listen, *flagTLS, *from, *websocket, *apiMax, handlers.Ratelimits{}, geometa{},
					fmt.Errorf("invalid -ratelimit flag: %q: %w", *ratelimit, v)
			}
			ratelimits.Set(name, int(r), s)
		}
	}

	return *dbConnect, *dbConn, *dev, *automigrate, *listen, *flagTLS, *from, *websocket, *apiMax, ratelimits, md, nil
}

func setupServe(dbConnect, dbConn string, dev bool, flagTLS string, automigrate bool) (zdb.DB, context.Context, *tls.Config, http.HandlerFunc, uint8, error) {
	if dev {
		setupReload()
	}

	db, ctx, err := connectDB(dbConnect, dbConn, map[bool][]string{true: {"all"}, false: {"pending"}}[automigrate], true, dev)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}

	ctx = z18n.With(ctx, z18n.NewBundle(language.English).Locale("en"))

	if dev {
		if !zio.Exists("db/migrate") || !zio.Exists("tpl") || !zio.Exists("public") {
			return nil, nil, nil, nil, 0, errors.New("-dev flag was given but this doesn't seem like a GoatCounter source directory")
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
						log.Errorf(ctx, "goatcounter was built from revision %s but source directory has revision %s", rev[:7], h[:7])
					}
				}
			}
		}
	}

	fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "tpl", dev)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}
	err = ztpl.Init(fsys)
	if err != nil {
		if !dev {
			return nil, nil, nil, nil, 0, err
		}
		log.Error(ctx, err)
	}

	tlsc, acmeh, listenTLS, secure := acme.Setup(db, flagTLS, dev)

	zhttp.CookieSecure = secure
	zhttp.ErrPage = handlers.ErrPage

	// Set SameSite=None to allow embedding GoatCounter in a frame and allowing
	// login; there is no way to make this work with Lax or Strict as far as I
	// can find (there is no way to add exceptions for trusted sites).
	//
	// This is not a huge problem because every POST/DELETE/etc. request already
	// has a CSRF token in the request, which protects against the same thing as
	// SameSite does. We could enable it only for sites that have "embed
	// GoatCounter" enabled (which aren't that many sites), but then people need
	// to logout and login again to reset the cookie, which isn't ideal.
	//
	// Only do this for secure connections, as Google Chrome developers decided
	// to silently reject these cookies if there's no TLS.
	if secure {
		zhttp.CookieSameSite = http.SameSiteNoneMode
	}

	err = goatcounter.Memstore.Init(db)
	if err != nil {
		return nil, nil, nil, nil, 0, err
	}

	cron.Start(goatcounter.CopyContextValues(ctx))
	return db, ctx, tlsc, acmeh, listenTLS, nil
}

func setupReload() {
	if _, err := os.Stat("./tpl"); os.IsNotExist(err) {
		return
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
		log.OnError = func(module string, r slog.Record) {
			bgrun.RunFunction("email:error", func() {
				buf := new(bytes.Buffer)
				h := slog_align.NewAlignedHandler(buf, &slog.HandlerOptions{
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == "module" || a.Key == "_err" {
							return slog.Attr{}
						}
						return a
					},
				})
				h.SetTimeFormat("Jan _2 15:04:05 ")
				h.SetColor(false)
				h.SetInlineLocation(false)
				h.Handle(context.Background(), r)

				msg := buf.String()

				// Silence spurious errors from some bot.
				if strings.Contains(msg, `ReferenceError: "Pikaday" is not defined.`) &&
					strings.Contains(msg, `Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.61 Safari/537.36`) {
					return
				}
				// Don't need to send notifications for these
				if strings.Contains(msg, `pq: canceling statement due to user request`) {
					return
				}

				subject := zstring.GetLine(msg, 1)
				if len(subject) > 15 { // Remove date: "Jun  8 00:51:41"
					subject = strings.TrimSpace(subject[15:])
				}
				subject = strings.TrimPrefix(subject, "ERROR")
				subject = strings.TrimLeft(subject, " \t:")

				err := blackmail.Send(subject,
					blackmail.From("", from),
					blackmail.To(to),
					blackmail.BodyText([]byte(msg)))
				if err != nil {
					// Just output to stderr I guess, can't really do much more if
					// sending email fails.
					fmt.Fprintf(zli.Stderr, "emailerrors: %s\n", err)
				}
			})
		}
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
			log.Errorf(ctx, "cname is empty for site %d/%s", s.ID, s.Code)
			continue
		}
		cnames = append(cnames, *s.Cname)
	}

	return cnames, nil
}

func startupAttr(geomd geometa, listen string, dev bool, attr ...any) []any {
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
			"path", geomd.path,
			"build", time.Unix(int64(geomd.md.BuildEpoch), 0).UTC().Format("2006-01-02 15:04:05"),
			"type", geomd.md.DatabaseType,
			"description", geomd.md.Description["en"],
			"nodes", geomd.md.NodeCount,
		),
	)
}

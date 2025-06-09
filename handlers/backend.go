package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sethvargo/go-limiter"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/log"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zstd/zfs"
)

func NewBackend(db zdb.DB, acmeh http.HandlerFunc, dev, goatcounterCom, websocket bool,
	domainStatic string, basePath string, dashTimeout, apiMax int, ratelimits Ratelimits,
) chi.Router {

	root := chi.NewRouter()
	r := root
	if basePath != "" {
		r = chi.NewRouter()
		root.Mount(basePath, r)
	}

	backend{dashTimeout, websocket}.Mount(r, db, dev, domainStatic, dashTimeout, apiMax, ratelimits)

	if acmeh != nil {
		r.Get("/.well-known/acme-challenge/{key}", acmeh)
	}

	if !goatcounterCom {
		NewStatic(r, dev, goatcounterCom, basePath)
	}

	return root
}

type backend struct {
	dashTimeout int
	websocket   bool
}

func (h backend) Mount(r chi.Router, db zdb.DB, dev bool, domainStatic string, dashTimeout, apiMax int, ratelimits Ratelimits) {
	if dev {
		r.Use(mware.Delay(0))
	}

	r.Use(
		mware.RealIP(),
		mware.WrapWriter(),
		mware.Unpanic("zgo.at/goatcounter/v2/handlers.add"),
		addctx(db, true, dashTimeout),
		addcsp(domainStatic),
		middleware.RedirectSlashes,
		mware.NoStore())
	if log.HasDebug("req") {
		r.Use(mware.RequestLog(nil, nil, "/count"))
	}
	if true {
		r.Use(middleware.NewCompressor(5).Handler)
	}

	fsys, err := zfs.EmbedOrDir(goatcounter.Templates, "", dev)
	if err != nil {
		panic(err)
	}
	static, err := zfs.EmbedOrDir(goatcounter.Static, "public", dev)
	if err != nil {
		panic(err)
	}

	website{fsys, false}.MountShared(r)
	newAPI(apiMax).mount(r, db, ratelimits)
	vcounter{static}.mount(r)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(404, T(r.Context(), "error/not-found|Not Found")))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		zhttp.ErrPage(w, r, guru.New(405, "Method Not Allowed"))
	})

	{
		rr := r.With(mware.Headers(nil))
		rr.Get("/robots.txt", zhttp.HandlerRobots([][]string{{"User-agent: *", "Disallow: /"}}))
		rr.Get("/security.txt", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
			return zhttp.Text(w, "Contact: support@goatcounter.com")
		}))
		rr.Post("/jserr", zhttp.HandlerJSErr())
		rr.Post("/csp", zhttp.HandlerCSP())

		// 4 pageviews/second should be more than enough.
		rate := rr.With(Ratelimit(true, func(r *http.Request) (limiter.Store, string) {
			if dev {
				return mustNewMem(1<<30, 1), ""
			}
			// From httpbuf
			// TODO: in some setups this may always be true, e.g. when proxy
			// through nginx without settings this properly. Need to check.
			if r.RemoteAddr == "127.0.0.1" {
				return mustNewMem(1<<14, 1), ""
			}
			return ratelimits.Count, ""
		}))
		rate.Get("/count", zhttp.Wrap(h.count))
		rate.Post("/count", zhttp.Wrap(h.count)) // to support navigator.sendBeacon (JS)
	}

	{
		{
			af := r.With(keyAuth, loggedIn, addz18n())
			bosmang{}.mount(af, db)
		}

		a := r.With(mware.Headers(http.Header{
			"Strict-Transport-Security": []string{"max-age=7776000"},
			"X-Content-Type-Options":    []string{"nosniff"},
			"X-Frame-Options":           []string{}, // Clear default from zhttp
		}), keyAuth, addz18n())
		user{}.mount(a, ratelimits)
		{
			ap := a.With(loggedInOrPublic, addz18n())
			ap.Get("/", zhttp.Wrap(h.dashboard))
			if h.websocket {
				ap.Get("/loader", zhttp.Wrap(h.loader))
			}
			ap.Get("/load-widget", zhttp.Wrap(h.loadWidget))
		}
		{
			af := a.With(loggedIn, addz18n())
			settings{}.mount(af, ratelimits)

			Newi18n().mount(af)
		}
	}
}

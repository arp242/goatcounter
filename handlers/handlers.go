package handlers

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/json"
	"zgo.at/z18n"
	"zgo.at/zhttp"
	"zgo.at/zstd/zfs"
	"zgo.at/ztpl"
)

func mustNewMem(tokens uint64, interval time.Duration) limiter.Store {
	s, err := memorystore.New(&memorystore.Config{Tokens: tokens, Interval: interval})
	if err != nil {
		// memorystore.New never returns an error, but just in case.
		panic(err)
	}
	return s
}

type Ratelimits struct {
	Count, API, APICount, Export, Login limiter.Store
}

func NewRatelimits() Ratelimits {
	return Ratelimits{
		Count:    mustNewMem(4, time.Second),
		API:      mustNewMem(4, time.Second),
		APICount: mustNewMem(60, 120*time.Second),
		Export:   mustNewMem(1, 3600*time.Second),
		Login:    mustNewMem(20, 60*time.Second),
	}
}

// Set the rate limits for name.
func (r *Ratelimits) Set(name string, tokens int, secs int64) {
	l := mustNewMem(uint64(tokens), time.Duration(secs)*time.Second)
	switch strings.ToLower(name) {
	case "count":
		r.Count = l
	case "api":
		r.API = l
	case "apicount", "api-count":
		r.APICount = l
	case "export":
		r.Export = l
	case "login":
		r.Login = l
	default:
		panic(fmt.Sprintf("handlers.SetRateLimit: invalid name: %q", name))
	}
}

// Site calls goatcounter.MustGetSite; it's just shorter :-)
func Site(ctx context.Context) *goatcounter.Site    { return goatcounter.MustGetSite(ctx) }
func Account(ctx context.Context) *goatcounter.Site { return goatcounter.MustGetAccount(ctx) }
func User(ctx context.Context) *goatcounter.User    { return goatcounter.MustGetUser(ctx) }

var T = z18n.T

type Globals struct {
	Context        context.Context
	User           *goatcounter.User
	Site           *goatcounter.Site
	Path           string
	Base           string
	Flash          *zhttp.FlashMessage
	Static         string
	StaticDomain   string
	Domain         string
	Version        string
	GoatcounterCom bool
	Dev            bool
	Port           string
	Websocket      bool
	JSTranslations map[string]string
	HideUI         bool
}

func (g Globals) T(msg string, data ...any) template.HTML {
	return template.HTML(z18n.T(g.Context, msg, data...))
}

func newGlobals(w http.ResponseWriter, r *http.Request) Globals {
	ctx := r.Context()
	base := goatcounter.Config(ctx).BasePath
	path := strings.TrimPrefix(r.URL.Path, base)
	if path == "" {
		path = "/"
	}
	g := Globals{
		Context:        ctx,
		User:           goatcounter.GetUser(ctx),
		Site:           goatcounter.GetSite(ctx),
		Path:           path,
		Base:           base,
		Flash:          zhttp.ReadFlash(w, r),
		Static:         goatcounter.Config(ctx).URLStatic,
		Domain:         goatcounter.Config(ctx).Domain,
		Version:        goatcounter.Version,
		GoatcounterCom: goatcounter.Config(ctx).GoatcounterCom,
		Dev:            goatcounter.Config(ctx).Dev,
		Port:           goatcounter.Config(ctx).Port,
		Websocket:      goatcounter.Config(ctx).Websocket,
		HideUI:         r.URL.Query().Get("hideui") != "",
		JSTranslations: map[string]string{
			"error/date-future":           T(ctx, "error/date-future|That would be in the future"),
			"error/date-past":             T(ctx, "error/date-past|That would be before the site’s creation; GoatCounter is not *that* good ;-)"),
			"error/date-mismatch":         T(ctx, "error/date-mismatch|end date is before start date"),
			"error/load-url":              T(ctx, "error/load-url|Could not load %(url): %(error)", z18n.P{"url": "%(url)", "error": "%(error)"}),
			"notify/saved":                T(ctx, "notify/saved|Saved!"),
			"dashboard/future":            T(ctx, "dashboard/future|future"),
			"dashboard/tooltip-event":     T(ctx, "dashboard/tooltip-event|%(unique) clicks; %(clicks) total clicks", z18n.P{"unique": "%(unique)", "clicks": "%(clicks)"}),
			"dashboard/totals/num-visits": T(ctx, "dashboard/totals/num-visits|%(num-visits) visits", z18n.P{"num-visits": "%(num-visits)"}),
			"datepicker/keyboard":         T(ctx, "datepicker/keyboard|Use the arrow keys to pick a date"),
			"datepicker/month-prev":       T(ctx, "datepicker/month-prev|Previous month"),
			"datepicker/month-next":       T(ctx, "datepicker/month-next|Next month"),
		},
	}
	if g.User == nil {
		g.User = &goatcounter.User{}
	}
	if goatcounter.Config(r.Context()).DomainStatic == "" {
		s := goatcounter.GetSite(r.Context())
		if s != nil {
			g.StaticDomain = s.Domain(r.Context())
		} else {
			g.StaticDomain = "/"
		}
	} else {
		g.StaticDomain = goatcounter.Config(r.Context()).DomainStatic
	}

	return g
}

func NewStatic(r chi.Router, dev, goatcounterCom bool, basePath string) chi.Router {
	var cache map[string]int
	if !dev {
		cache = map[string]int{
			"/count.js": 86400,
			"*":         86400 * 30,
		}
	}
	fsys, err := zfs.EmbedOrDir(goatcounter.Static, "public", dev)
	if err != nil {
		panic(err)
	}

	s := zhttp.NewStatic("*", fsys, cache)
	s.Header("/count.js", map[string]string{
		"Cross-Origin-Resource-Policy": "cross-origin",
	})
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, basePath)
		s.ServeHTTP(w, r)
	})
	return r
}

// Identical to the default errpage, but replaces slog calls with our log calls.
func ErrPage(w http.ResponseWriter, r *http.Request, reported error) {
	if reported == nil {
		return
	}
	hasStatus := true
	if ww, ok := w.(statusWriter); !ok || ww.Status() == 0 {
		hasStatus = false
	}

	code, userErr := zhttp.UserError(reported)
	if code >= 500 {
		l := log.Module("http-500")
		l = l.With("code", zhttp.UserErrorCode(reported))

		sErr := new(interface{ StackTrace() string })
		if errors.As(reported, sErr) {
			reported = errors.Unwrap(reported)
			l = l.With("stacktrace", "\n"+(*sErr).StackTrace())
		}
		l.Error(r.Context(), reported, log.AttrHTTP(r))
	}

	ct := strings.ToLower(r.Header.Get("Content-Type"))
	switch {
	case strings.HasPrefix(ct, "application/json"):
		if !hasStatus {
			w.WriteHeader(code)
		}

		var (
			j   []byte
			err error
		)

		if jErr, ok := userErr.(json.Marshaler); ok {
			j, err = jErr.MarshalJSON()
		} else if jErr, ok := userErr.(interface{ ErrorJSON() ([]byte, error) }); ok {
			j, err = jErr.ErrorJSON()
		} else {
			j, err = json.Marshal(map[string]string{"error": userErr.Error()})
		}
		if err != nil {
			log.Error(r.Context(), err, log.AttrHTTP(r))
		}
		w.Write(j)

	case strings.HasPrefix(ct, "text/plain"):
		if !hasStatus {
			w.WriteHeader(code)
		}
		fmt.Fprintf(w, "Error %d: %s", code, userErr)

	case !hasStatus && r.Referer() != "" && ct == "application/x-www-form-urlencoded" || strings.HasPrefix(ct, "multipart/"):
		zhttp.FlashError(w, userErr.Error())
		zhttp.SeeOther(w, r.Referer())

	default:
		if !hasStatus {
			w.WriteHeader(code)
		}

		if !ztpl.HasTemplate("error.gohtml") {
			fmt.Fprintf(w, "<pre>Error %d: %s</pre>", code, userErr)
			return
		}

		err := ztpl.Execute(w, "error.gohtml", struct {
			Code  int
			Error error
			Base  string
			Path  string
		}{code, userErr, zhttp.BasePath, r.URL.Path})
		if err != nil {
			log.Error(r.Context(), err, log.AttrHTTP(r))
		}
	}
}

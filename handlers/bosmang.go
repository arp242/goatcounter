package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/pkg/bgrun"
	"zgo.at/goatcounter/v2/pkg/geo"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/goatcounter/v2/pkg/metrics"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zprof"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
	"zgo.at/zvalidate"
)

type bosmang struct{}

func (h bosmang) mount(r chi.Router, db zdb.DB) {
	a := r.With(mware.RequestLog(nil, nil), requireAccess(goatcounter.AccessSuperuser))

	r.Get("/bosmang", zhttp.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.MovedPermanently(w, "/settings/server")
	}))

	a.Get("/bosmang/cache", zhttp.Wrap(h.cache))
	a.Get("/bosmang/error", zhttp.Wrap(h.error))
	a.Get("/bosmang/bgrun", zhttp.Wrap(h.bgrun))
	a.Get("/bosmang/email", zhttp.Wrap(h.email))
	a.Post("/bosmang/email", zhttp.Wrap(h.sendEmail))
	a.Get("/bosmang/geoip", zhttp.Wrap(h.geoip))
	a.Post("/bosmang/bgrun/{task}", zhttp.Wrap(h.runTask))
	a.Get("/bosmang/metrics", zhttp.Wrap(h.metrics))
	a.Handle("/bosmang/profile*", zprof.NewHandler(zprof.Prefix("/bosmang/profile")))
}

func (h bosmang) cache(w http.ResponseWriter, r *http.Request) error {
	cache := goatcounter.ListCache(r.Context())
	return zhttp.Template(w, "bosmang_cache.gohtml", struct {
		Globals
		Cache map[string]struct {
			Size  int64
			Items map[string]string
		}
	}{newGlobals(w, r), cache})
}

func (h bosmang) bgrun(w http.ResponseWriter, r *http.Request) error {
	hist := bgrun.History(0)

	metrics := make(map[string]*ztime.Durations)
	for _, h := range hist {
		x, ok := metrics[h.Task]
		if !ok {
			x = ztype.Ptr(ztime.NewDurations(0))
			x.Grow(32)
		}
		x.Append(h.Took)
		metrics[h.Task] = x
	}

	return zhttp.Template(w, "bosmang_bgrun.gohtml", struct {
		Globals
		Tasks   []cron.Task
		Jobs    []bgrun.Job
		History []bgrun.Job
		Metrics map[string]*ztime.Durations
	}{newGlobals(w, r), cron.Tasks, bgrun.Running(), hist, metrics})
}

func (h bosmang) runTask(w http.ResponseWriter, r *http.Request) error {
	v := zvalidate.New()
	taskID := v.Integer("task", chi.URLParam(r, "task"))
	v.Range("task", taskID, 0, int64(len(cron.Tasks)-1))
	if v.HasErrors() {
		return v
	}

	t := cron.Tasks[taskID]
	id := t.ID()
	ctx := context.WithoutCancel(r.Context())
	bgrun.RunFunction("manual:"+id, func() {
		err := t.Fun(ctx)
		if err != nil {
			log.Error(ctx, err)
		}
	})

	zhttp.Flash(w, r, fmt.Sprintf("Task %q started", id))
	return zhttp.SeeOther(w, "/bosmang/bgrun")
}

func (h bosmang) metrics(w http.ResponseWriter, r *http.Request) error {
	by := "sum"
	if b := r.URL.Query().Get("by"); b != "" {
		by = b
	}
	return zhttp.Template(w, "bosmang_metrics.gohtml", struct {
		Globals
		Metrics metrics.Metrics
		By      string
	}{newGlobals(w, r), metrics.List().Sort(by), by})
}

func (h bosmang) error(w http.ResponseWriter, r *http.Request) error {
	return guru.New(500, "test error")
}

func (h bosmang) emailTpl(w http.ResponseWriter, r *http.Request, email string, err error, out string) error {
	return zhttp.Template(w, "bosmang_email.gohtml", struct {
		Globals
		Info       map[string]any
		From       string
		Email      string
		TestErr    error
		TestResult string
	}{newGlobals(w, r), blackmail.Get(r.Context()).Info(),
		goatcounter.Config(r.Context()).EmailFrom,
		email, err, out,
	})
}

func (h bosmang) email(w http.ResponseWriter, r *http.Request) error {
	return h.emailTpl(w, r, goatcounter.Config(r.Context()).EmailFrom, nil, "")
}

func (h bosmang) sendEmail(w http.ResponseWriter, r *http.Request) error {
	var (
		email = r.Form.Get("email")
		m     = blackmail.Get(r.Context())
		info  = m.Info()
		dbg   = new(bytes.Buffer)
	)
	switch info["sender"] {
	case "mailerRelay":
		o := info["opts"].(blackmail.RelayOptions)
		if o.Debug != nil {
			o.Debug = io.MultiWriter(o.Debug, dbg)
		} else {
			o.Debug = dbg
		}
		var err error
		m, err = blackmail.NewRelay(info["url"].(string), &o)
		if err != nil {
			return err
		}
	case "mailerWriter":
		fmt.Fprintf(dbg, "mailerWriter: wrote email to fd %s (%s)", info["fd"], info["name"])
	}

	err := m.Send(
		"GoatCounter test email",
		blackmail.From("GoatCounter", goatcounter.Config(r.Context()).EmailFrom),
		blackmail.To(email),
		blackmail.HeadersAutoreply(),
		blackmail.BodyText([]byte("Test email from GoatCounter")))
	return h.emailTpl(w, r, email, err, dbg.String())
}

func (h bosmang) geoip(w http.ResponseWriter, r *http.Request) error {
	var (
		geodb = geo.Get(r.Context())
		md    = geodb.DB().Metadata
		l     goatcounter.Location
	)
	lookupErr := l.Lookup(r.Context(), r.RemoteAddr)

	type GeoDB struct {
		Path        string
		Build       time.Time
		Type        string
		Description string
		Nodes       uint
	}
	return zhttp.Template(w, "bosmang_geoip.gohtml", struct {
		Globals
		IP          string
		Header      http.Header
		Location    goatcounter.Location
		Error       error
		ShowHeaders []string
		GeoDB       GeoDB
	}{newGlobals(w, r), r.RemoteAddr, r.Header, l, lookupErr,
		[]string{"Cf-Connecting-Ip", "Fly-Client-Ip", "X-Azure-Socketip", "X-Real-Ip", "X-Forwarded-For"},
		GeoDB{
			geodb.DB().Path,
			time.Unix(int64(md.BuildEpoch), 0).UTC(),
			md.DatabaseType,
			md.Description["en"],
			md.NodeCount,
		},
	})
}

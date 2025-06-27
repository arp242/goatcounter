package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/pkg/bgrun"
	"zgo.at/goatcounter/v2/pkg/log"
	"zgo.at/goatcounter/v2/pkg/metrics"
	"zgo.at/guru"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zprof"
	"zgo.at/zstd/zcontext"
	"zgo.at/zstd/ztime"
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

	metrics := make(map[string]ztime.Durations)
	for _, h := range hist {
		x, ok := metrics[h.Task]
		if !ok {
			x = ztime.NewDurations(0)
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
		Metrics map[string]ztime.Durations
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
	ctx := zcontext.WithoutTimeout(r.Context())
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

package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"
	"zgo.at/goatcounter"
	"zgo.at/zhttp"
	"zgo.at/zlog"

	"zgo.at/goatcounter/cfg"
)

type Backend struct{}

func (h Backend) Mount(r chi.Router, db *sqlx.DB) {
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		addctx(db, true),
		zhttp.Headers(nil),
		zhttp.Log(true, ""))

	r.Get("/count", zhttp.Wrap(h.count))

	a := r.With(keyAuth)

	// Backend interface.
	a.Get("/", zhttp.Wrap(h.index))
	a.Get("/refs", zhttp.Wrap(h.refs))

	// Backend.
	user{}.mount(a)
}

func (h Backend) count(w http.ResponseWriter, r *http.Request) error {
	var t goatcounter.Hit
	_, err := zhttp.Decode(r, &t)
	if err != nil {
		return err
	}

	// TODO: filter stuff from localhost

	err = t.Insert(r.Context())
	if err != nil {
		zlog.Error(err)
		return err
	}

	w.Header().Set("Cache-Control", "no-store,no-cache")
	return zhttp.String(w, "")
}

const day = 24 * time.Hour

func (h Backend) index(w http.ResponseWriter, r *http.Request) error {
	// TODO: Use period first as fallback when there's no JS.
	// p := r.URL.Query().Get("period")

	start := time.Now().Add(-7 * day)
	if s := r.URL.Query().Get("period-start"); s != "" {
		var err error
		start, err = time.Parse("2006-01-02", s)
		if err != nil {
			return err
		}
	}
	end := time.Now()
	if s := r.URL.Query().Get("period-end"); s != "" {
		var err error
		end, err = time.Parse("2006-01-02", s)
		if err != nil {
			return err
		}
	}

	var pages goatcounter.HitStats
	err := pages.List(r.Context(), start, end)
	if err != nil {
		return err
	}

	// Add refers.
	sr := r.URL.Query().Get("showrefs")
	var refs goatcounter.HitStats
	if sr != "" {
		err := refs.ListRefs(r.Context(), sr, start, end)
		if err != nil {
			return err
		}
	}

	return zhttp.Template(w, "backend.gohtml", struct {
		Globals
		ShowRefs    string
		PeriodStart time.Time
		PeriodEnd   time.Time
		Pages       goatcounter.HitStats
		Refs        goatcounter.HitStats
	}{newGlobals(w, r), sr, start, end, pages, refs})
}

func (h Backend) refs(w http.ResponseWriter, r *http.Request) error {
	start, err := time.Parse("2006-01-02", r.URL.Query().Get("period-start"))
	if err != nil {
		return err
	}

	end, err := time.Parse("2006-01-02", r.URL.Query().Get("period-end"))
	if err != nil {
		return err
	}

	var refs goatcounter.HitStats
	err = refs.ListRefs(r.Context(), r.URL.Query().Get("showrefs"), start, end)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "_backend_refs.gohtml", refs)
}

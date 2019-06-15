package handlers

import (
	"net/http"
	"strconv"

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

	// Backend.
	user{}.mount(a)
}

func (h Backend) count(w http.ResponseWriter, r *http.Request) error {
	var t goatcounter.Hit
	_, err := zhttp.Decode(r, &t)
	if err != nil {
		zlog.Error(err)
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

func (h Backend) index(w http.ResponseWriter, r *http.Request) error {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		dd, _ := strconv.ParseInt(d, 10, 32)
		days = int(dd)
	}
	_ = days

	var pages goatcounter.HitStats
	err := pages.List(r.Context())
	if err != nil {
		return err
	}

	sr := r.URL.Query().Get("showrefs")
	var refs goatcounter.HitStats
	if sr != "" {
		err := refs.ListPath(r.Context(), sr)
		if err != nil {
			return err
		}
	}

	/*
		var top goatcounter.HitList
		err := top.List(r.Context())
		if err != nil {
			return err
		}

		var hits goatcounter.HitStats
		_, err = hits.Hourly(r.Context(), days)
		if err != nil {
			return err
		}

		var refs goatcounter.RefStats
		err = refs.List(r.Context())
		if err != nil {
			return err
		}
	*/

	return zhttp.Template(w, "backend.gohtml", struct {
		Globals
		ShowRefs string
		Pages    goatcounter.HitStats
		Refs     goatcounter.HitStats
		//HitStats goatcounter.HitStats
		//RefStats goatcounter.RefStats
	}{newGlobals(w, r), sr, pages, refs})
}

package handlers // import "zgo.at/goatcounter/handlers"

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"

	"zgo.at/goatcounter/cfg"
	"zgo.at/zhttp"
)

type Website struct{}

func (h Website) Mount(r *chi.Mux, db *sqlx.DB) {
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		middleware.RedirectSlashes,
		addctx(db, false),
		zhttp.Headers(nil),
		zhttp.Log(true, ""),
		keyAuth)

	r.Get("/", zhttp.Wrap(h.home))
	r.Get("/status", zhttp.Wrap(h.status()))
	r.Get("/privacy", zhttp.Wrap(h.privacy))
	user{}.mount(r)
}

func (h Website) home(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "home.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

func (h Website) privacy(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "privacy.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

func (h Website) status() func(w http.ResponseWriter, r *http.Request) error {
	started := time.Now()
	return func(w http.ResponseWriter, r *http.Request) error {
		return zhttp.JSON(w, map[string]string{
			"uptime":  time.Now().Sub(started).String(),
			"version": cfg.Version,
		})
	}
}

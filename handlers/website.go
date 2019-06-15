package handlers // import "zgo.at/goatcounter/handlers"

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/jmoiron/sqlx"

	"zgo.at/goatcounter/cfg"
	"zgo.at/zhttp"
)

type Website struct{}

func (h Website) Mount(r *chi.Mux, db *sqlx.DB) {
	// TODO: remove trailing slashes (/foo/ won't work).
	// TODO: rate limit to some degree: https://github.com/Teamwork/middleware/tree/master/ratelimit
	r.Use(
		middleware.RealIP,
		zhttp.Unpanic(cfg.Prod),
		addctx(db, false),
		zhttp.Headers(nil),
		zhttp.Log(true, ""),
		keyAuth)

	r.Get("/", zhttp.Wrap(h.home))
	user{}.mount(r)
}

func (h Website) home(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "home.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

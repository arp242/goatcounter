// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

var (
	redirect = func(w http.ResponseWriter, r *http.Request) error {
		zhttp.Flash(w, "Need to log in")
		return guru.Errorf(303, "/user/new")
	}

	loggedIn = zhttp.Filter(func(w http.ResponseWriter, r *http.Request) error {
		u := goatcounter.GetUser(r.Context())
		if u != nil && u.ID > 0 {
			return nil
		}
		return redirect(w, r)
	})

	loggedInOrPublic = zhttp.Filter(func(w http.ResponseWriter, r *http.Request) error {
		u := goatcounter.GetUser(r.Context())
		if (u != nil && u.ID > 0) || goatcounter.MustGetSite(r.Context()).Settings.Public {
			return nil
		}
		return redirect(w, r)
	})

	admin = zhttp.Filter(func(w http.ResponseWriter, r *http.Request) error {
		if goatcounter.MustGetSite(r.Context()).ID == 1 {
			return nil
		}
		return guru.Errorf(404, "")
	})

	keyAuth = zhttp.Auth(func(ctx context.Context, key string) (zhttp.User, error) {
		u := &goatcounter.User{}
		err := u.ByToken(ctx, key)
		return u, err
	})
)

func addctx(db *sqlx.DB, loadSite bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add database and timeout to context.
			ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer func() {
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					w.WriteHeader(http.StatusGatewayTimeout)
				}
			}()

			// Add database.
			*r = *r.WithContext(zdb.With(ctx, db))

			// Load site from subdomain
			if loadSite {
				var s goatcounter.Site
				err := s.ByHost(r.Context(), r.Host)
				if err != nil {
					if errors.Cause(err) == sql.ErrNoRows {
						zhttp.ErrPage(w, r, 400, fmt.Errorf("no site at this domain (%q)", r.Host))
						return
					}

					zlog.Error(err)
					return
				}

				*r = *r.WithContext(context.WithValue(r.Context(), ctxkey.Site, &s))
			}

			next.ServeHTTP(w, r)
		})
	}
}

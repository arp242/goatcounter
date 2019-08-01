// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"zgo.at/goatcounter"
	"zgo.at/zhttp"
	"zgo.at/zhttp/ctxkey"
	"zgo.at/zlog"
)

var (
	filterLoggedIn = zhttp.Filter(func(r *http.Request) bool {
		u := goatcounter.GetUser(r.Context())
		return u != nil && u.ID > 0
	})

	// emailAuth = zhttp.Auth(func(ctx context.Context, email string) (zhttp.User, error) {
	// 	u := &goatcounter.User{}
	// 	err := u.ByEmail(ctx, email)
	// 	return u, err

	// })

	keyAuth = zhttp.Auth(func(ctx context.Context, key string) (zhttp.User, error) {
		u := &goatcounter.User{}
		err := u.ByKey(ctx, key)
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
			r = r.WithContext(context.WithValue(ctx, ctxkey.DB, db))

			// Load site from subdomain
			if loadSite {
				i := strings.Index(r.Host, ".")
				if i == -1 {
					zhttp.ErrPage(w, r, 400, fmt.Errorf("no subdomain in host %q", r.Host))
					return
				}

				var s goatcounter.Site
				err := s.ByCode(r.Context(), r.Host[:i])
				if err != nil {
					if errors.Cause(err) == sql.ErrNoRows {
						zhttp.ErrPage(w, r, 400, fmt.Errorf("no site at this domain (%q)", r.Host[:i]))
						return
					}

					zlog.Error(err)
					return
				}

				r = r.WithContext(context.WithValue(r.Context(), ctxkey.Site, &s))
			}

			next.ServeHTTP(w, r)
		})
	}
}

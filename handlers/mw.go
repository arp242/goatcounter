// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/guru"
	"zgo.at/json"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/auth"
	"zgo.at/zlog"
	"zgo.at/zstd/znet"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zstring"
)

var (
	redirect = func(w http.ResponseWriter, r *http.Request) error {
		zhttp.Flash(w, "Need to log in")
		return guru.Errorf(303, "/user/new")
	}

	loggedIn = auth.Filter(func(w http.ResponseWriter, r *http.Request) error {
		u := goatcounter.GetUser(r.Context())
		if u != nil && u.ID > 0 {
			return nil
		}
		return redirect(w, r)
	})

	loggedInOrPublic = auth.Filter(func(w http.ResponseWriter, r *http.Request) error {
		u := goatcounter.GetUser(r.Context())
		if (u != nil && u.ID > 0) || Site(r.Context()).Settings.Public {
			return nil
		}
		return redirect(w, r)
	})

	adminOnly = auth.Filter(func(w http.ResponseWriter, r *http.Request) error {
		if Site(r.Context()).Admin() {
			return nil
		}
		return guru.Errorf(404, "")
	})

	keyAuth = auth.Add(func(ctx context.Context, key string) (auth.User, error) {
		u := &goatcounter.User{}
		err := u.ByTokenAndSite(ctx, key)
		return u, err
	})
)

type statusWriter interface{ Status() int }

func addctx(db zdb.DB, loadSite bool) func(http.Handler) http.Handler {
	started := goatcounter.Now()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if r.URL.Path == "/status" {
				v, _ := zdb.MustGetDB(ctx).Version(ctx)
				j, err := json.Marshal(map[string]interface{}{
					"uptime":            goatcounter.Now().Sub(started).Round(time.Second).String(),
					"version":           goatcounter.Version,
					"last_persisted_at": cron.LastMemstore.Get().Format(time.RFC3339Nano),
					"database":          zdb.Driver(ctx).String() + " " + string(v),
					"go":                runtime.Version(),
					"GOOS":              runtime.GOOS,
					"GOARCH":            runtime.GOARCH,
					"race":              zruntime.Race,
					"cgo":               zruntime.CGO,
				})
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)

				w.Write(j)
				return
			}

			// Add timeout on non-admin pages.
			t := 3
			switch {
			case strings.HasPrefix(r.URL.Path, "/admin"):
				t = 120
			case r.URL.Path == "/":
				t = 11
			}
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(r.Context(), time.Duration(t)*time.Second)
			defer func() {
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					if ww, ok := w.(statusWriter); !ok || ww.Status() == 0 {
						w.WriteHeader(http.StatusGatewayTimeout)
						w.Write([]byte("Server timed out"))
					}
				}
			}()

			// Wrap in explainDB for testing.
			if goatcounter.Config(r.Context()).Dev {
				if c, _ := r.Cookie("debug-explain"); c != nil {
					*r = *r.WithContext(zdb.WithDB(ctx, zdb.NewLogDB(zdb.MustGetDB(ctx),
						os.Stdout, zdb.DumpQuery|zdb.DumpExplain, c.Value)))
				}
			}

			// Load site from subdomain.
			if loadSite {
				var s goatcounter.Site
				err := s.ByHost(r.Context(), r.Host)

				if err != nil && goatcounter.Config(r.Context()).Serve {
					// If there's just one site then we can just serve that;
					// most people probably have just one site so it's all
					// grand.
					//
					// Do print a warning in the console though.
					var sites goatcounter.Sites
					err2 := sites.UnscopedList(r.Context())
					if err2 == nil && len(sites) == 1 {
						s = sites[0]
						err = nil

						if r.URL.Path == "/" {
							zlog.Printf(zstring.WordWrap(fmt.Sprintf(""+
								"accessing the site on domain %q, but the configured domain is %q; "+
								"this will work fine as long as you only have one site, but you *need* to use the "+
								"configured domain if you add a second site so GoatCounter will know which site to use.",
								znet.RemovePort(r.Host), *s.Cname), strings.Repeat(" ", 25), 55))
						}
					}
					if err2 == nil && len(sites) == 0 {
						err = guru.Errorf(400, ""+
							`no sites created yet; create a new site from the commandline with `+
							`"goatcounter create -domain [..] -email [..]"`)
					}
				}

				if err != nil {
					if zdb.ErrNoRows(err) {
						err = guru.Errorf(400, "no site at this domain (%q)", r.Host)
					} else {
						zlog.FieldsRequest(r).Error(err)
					}

					zhttp.ErrPage(w, r, err)
					return
				}

				*r = *r.WithContext(goatcounter.WithSite(r.Context(), &s))
			}

			next.ServeHTTP(w, r)
		})
	}
}

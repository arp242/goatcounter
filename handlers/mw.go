// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/guru"
	"zgo.at/json"
	"zgo.at/termtext"
	"zgo.at/z18n"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zhttp/auth"
	"zgo.at/zlog"
	"zgo.at/zstd/znet"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/ztime"
)

// Started is set when the server is started.
var Started time.Time

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
		if u != nil && u.ID > 0 {
			return nil
		}
		s := Site(r.Context())
		if s.Settings.IsPublic() {
			return nil
		}
		if a := r.URL.Query().Get("access-token"); s.Settings.CanView(a) {
			// Set cookie for auth and redirect. This prevents accidental
			// leaking of the secret by copy/pasting the URL, screenshots, etc.
			http.SetCookie(w, &http.Cookie{
				Name:     "access-token",
				Value:    a,
				Path:     "/",
				HttpOnly: true,
				Secure:   zhttp.CookieSecure,
				SameSite: zhttp.CookieSameSite,
			})
			return guru.Errorf(303, "/")
		}
		if c, err := r.Cookie("access-token"); err == nil && s.Settings.CanView(c.Value) {
			return nil
		}

		return redirect(w, r)
	})

	requireAccess = func(atLeast goatcounter.UserAccess) func(http.Handler) http.Handler {
		return auth.Filter(func(w http.ResponseWriter, r *http.Request) error {
			u := goatcounter.GetUser(r.Context())
			if u != nil && u.ID > 0 && u.HasAccess(atLeast) {
				return nil
			}
			return guru.Errorf(401, "Not allowed to view this page")
		})
	}

	keyAuth = auth.Add(func(ctx context.Context, key string) (auth.User, error) {
		u := &goatcounter.User{}
		err := u.ByTokenAndSite(ctx, key)
		return u, err
	}, "/bosmang/profile/setrate")
)

type statusWriter interface{ Status() int }

func addctx(db zdb.DB, loadSite bool, dashTimeout int) func(http.Handler) http.Handler {
	Started = ztime.Now()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			if r.URL.Path == "/status" {
				info, _ := zdb.Info(ctx)
				j, err := json.Marshal(map[string]interface{}{
					"uptime":            ztime.Now().Sub(Started).Round(time.Second).String(),
					"version":           goatcounter.Version,
					"last_persisted_at": cron.LastMemstore.Get().Round(time.Second).Format(time.RFC3339),
					"database":          zdb.SQLDialect(ctx).String() + " " + string(info.Version),
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
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.WriteHeader(200)

				w.Write(j)
				return
			}

			// Add timeout.
			t := 3
			if r.URL.Path == "/" {
				t = dashTimeout + 1
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
						os.Stderr, zdb.DumpQuery|zdb.DumpExplain, c.Value)))
				}
				if c, _ := r.Cookie("debug-dump"); c != nil {
					*r = *r.WithContext(zdb.WithDB(ctx, zdb.NewLogDB(zdb.MustGetDB(ctx),
						os.Stderr, zdb.DumpQuery|zdb.DumpResult, c.Value)))
				}
			}

			// Load site from domain.
			if loadSite {
				var s goatcounter.Site // code
				err := s.ByHost(r.Context(), r.Host)

				// If there's just one site then we can just serve that; most
				// people probably have just one site so it's all grand. Do
				// print a warning in the console though.
				if err != nil && !goatcounter.Config(r.Context()).GoatcounterCom {
					var sites goatcounter.Sites
					err2 := sites.UnscopedList(r.Context())
					if err2 == nil && len(sites) == 1 {
						s = sites[0]
						err = nil

						if r.URL.Path == "/" {
							txt := fmt.Sprintf(""+
								"accessing the site on domain %q, but the configured domain is %q; "+
								"this will work fine as long as you only have one site, but you *need* to use the "+
								"configured domain if you add a second site so GoatCounter will know which site to use.",
								znet.RemovePort(r.Host), *s.Cname)
							zlog.Printf(termtext.WordWrap(txt, 55, strings.Repeat(" ", 25)))
						}
					}
					if err2 == nil && len(sites) == 0 {
						err = guru.Errorf(400, ""+
							`no sites created yet; create a new site from the commandline with `+
							`"goatcounter db create site -vhost=.. -user.email=.."`)
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

			// Make sure there's always a z18n object.
			*r = *r.WithContext(z18n.With(r.Context(), goatcounter.GetBundle(r.Context()).Locale(r.Header.Get("Accept-Language"))))

			next.ServeHTTP(w, r)
		})
	}
}

func addz18n() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var siteLang, userLang string
			if s := goatcounter.GetSite(r.Context()); s != nil {
				siteLang = s.UserDefaults.Language
			}
			if u := goatcounter.GetUser(r.Context()); u != nil {
				userLang = u.Settings.Language
			}
			*r = *r.WithContext(z18n.With(r.Context(), goatcounter.GetBundle(r.Context()).Locale(userLang, siteLang, r.Header.Get("Accept-Language"))))
			next.ServeHTTP(w, r)
		})
	}
}

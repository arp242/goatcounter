// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/monoculum/formam"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/handlers"
	"zgo.at/json"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zsync"
)

const usageBuffer = `
The buffer command accepts requests for /count and will send them to a
GoatCounter backend when it's available. This is useful as a lightweight
backup/redundancy solution to protect against server crashes or to ensure
pageviews are still recorded during database migrations, server moves, etc.

The general way to use this is to check if the main goatcounter backend is
available in a proxy, and use the buffer backend if it's not; for example with
Varnish

        backend goatcounter {
            .host = "127.0.0.1";
            .port = "8081";
        }
        backend buffer {
            .host = "127.0.0.1";
            .port = "8082";
        }

        sub vcl_backend_error {
            if (bereq.url ~ "^/count") {
                if (bereq.retries >= 3) {
                    set bereq.backend = buffer;
                    return(retry);
                }

                vtc.sleep(300ms * (bereq.retries + 1));
                return(retry);
            }
        }

Requests are stored in memory only; a single request takes about 1.5K of memory,
so buffering 10,000 requests takes about 15M of memory. If you want a persistent
disk-based backup you can tell the proxy to log the requests to a file and
import them from there (you can of course do both as a double redundancy).

The requests are forwarded as a regular HTTP request with a secret key to bypass
the rate limiter and set the correct created date. To generate this key, use:

    $ goatcounter buffer -generate-key

Only the -generate-key command requires access to the database. You can also
insert it manually from SQL with:

    insert into store (key, value) values ('buffer-secret', 'your-secret')

A special /_list endpoint can be used to display the contents of the buffer.

Flags:

  -generate-key  Create a new secret key. This will invalidate any previously
                 generated key.

  -db          Database connection: "sqlite+<file>" or "postgres+<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite+/db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

               Only needed for -generate-key

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

  -silent      Don't show informational messages about the buffer size.

  -listen      Listen address. Default: localhost:8082

  -backend     GoatCounter backend as an URL. Default: https://localhost

  -bufsize     Maximum amount of requests to store; requests will be refused with
               a 429 code if the buffer is at the maximum size. Default: 500,000
               (requires ~725M of memory).

Environment:

  GOATCOUNTER_BUFFER_SECRET   Secret to use to identify the buffered requests.
`

var (
	bufCheckBackendTime = 10 * time.Second
	bufSendTime         = 3 * time.Second
)

func cmdBuffer(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	var (
		isDown    = zsync.NewAtomicInt(-1)
		reqBuffer chan handlers.APICountRequestHit
		bufClient = &http.Client{Timeout: 3 * time.Second}
	)

	var (
		dbConnect = f.String("sqlite+db/goatcounter.sqlite3", "db").Pointer()
		debug     = f.String("", "debug").Pointer()
		listen    = f.String("localhost:8082", "listen").Pointer()
		backend   = f.String("https://localhost", "backend").Pointer()
		bufSize   = f.Int(500_000, "bufsize").Pointer()
		silent    = f.Bool(false, "silent").Pointer()
		genKey    = f.Bool(false, "generate-key").Pointer()
	)
	err := f.Parse()
	if err != nil {
		return err
	}

	return func(dbConnect, debug, listen, backend string, bufSize int, silent, genKey bool) error {
		zlog.Config.SetDebug(debug)

		key := os.Getenv("GOATCOUNTER_BUFFER_SECRET")
		if key == "" && !genKey {
			return errors.New("need to set GOATCOUNTER_BUFFER_SECRET; use 'goatcounter buffer -generate-key' to create a new one")
		}

		if genKey {
			ready <- struct{}{}
			db, ctx, err := connectDB(dbConnect, []string{"pending"}, false, false)
			if err != nil {
				return err
			}
			defer db.Close()

			secret, err := goatcounter.NewBufferKey(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintln(zli.Stdout, "Your new secret key is:")
			fmt.Fprintln(zli.Stdout, secret)
			return nil
		}

		reqBuffer = make(chan handlers.APICountRequestHit, bufSize)

		// Ping backend status.
		go func() {
			defer zlog.Recover()
			checkURL := backend + "/status"
			for {
				checkBackend(bufClient, checkURL, isDown)
				time.Sleep(bufCheckBackendTime)
			}
		}()

		// Send buffered requests.
		go func() {
			defer zlog.Recover()

			for {
				time.Sleep(bufSendTime)

				if isDown.Value() != 0 {
					continue
				}

				l := len(reqBuffer)
				if l == 0 {
					continue
				}
				if l > 100 {
					l = 100
				}

				grouped := make(map[string][]handlers.APICountRequestHit)
				for i := 0; i < l; i++ {
					h := <-reqBuffer
					grouped[h.Host] = append(grouped[h.Host], h)
				}

				for host, hits := range grouped {
					j, err := json.Marshal(handlers.APICountRequest{Hits: hits})
					if err != nil {
						zlog.Error(err)
						continue
					}

					r, err := newRequest("POST", backend+"/api/v0/count", key, bytes.NewReader(j))
					if err != nil {
						zlog.Error(err)
						continue
					}

					r.Host = host
					r.Header.Set("X-Goatcounter-Buffer", "1")
					resp, err := bufClient.Do(r)
					if err != nil {
						zlog.Error(err)
						continue
					}

					if resp.StatusCode >= 300 {
						b, _ := io.ReadAll(resp.Body)
						zlog.Errorf("  Sending %s FAILED: %s\n%s", r.URL, resp.Status, b)
					} else if !silent {
						zlog.Printf("  Sending %s OKAY\n", r.URL)
					}
					resp.Body.Close()
				}
			}
		}()

		zlog.Printf("Ready on %s", listen)
		ch, err := zhttp.Serve(0, stop, &http.Server{
			Addr:              listen,
			Handler:           mware.RealIP()(mware.Unpanic()(handleBuffer(reqBuffer, bufClient, isDown, silent))),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       60 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		})
		if err != nil {
			return nil
		}

		<-ch
		ready <- struct{}{}
		<-ch
		return nil
	}(*dbConnect, *debug, *listen, *backend, *bufSize, *silent, *genKey)
}

// Collect all requests.
func handleBuffer(
	reqBuffer chan handlers.APICountRequestHit, bufClient *http.Client, isDown *zsync.AtomicInt, silent bool,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_list" {
			all := make([]handlers.APICountRequestHit, 0, len(reqBuffer))
			fmt.Fprintf(os.Stderr, "buffer len: %d\n", len(reqBuffer))
			for len(reqBuffer) > 0 {
				h := <-reqBuffer
				all = append(all, h)
				fmt.Fprintln(os.Stderr, h)
			}
			for _, h := range all {
				reqBuffer <- h
			}

			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "buffer len: %d\n", len(reqBuffer))
			fmt.Fprintf(w, "backend status: %d\n", isDown.Value())
			fmt.Fprintln(w, "contents dumped to stderr")
			return
		}

		if len(reqBuffer) == cap(reqBuffer) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		var hit handlers.APICountRequestHit
		err := formam.NewDecoder(&formam.DecoderOptions{
			TagName:           "query",
			IgnoreUnknownKeys: true,
		}).Decode(r.URL.Query(), &hit)
		if err != nil {
			zlog.Error(err)
			http.Error(w, err.Error(), 400)
			return
		}

		hit.UserAgent = r.UserAgent()
		hit.IP = r.RemoteAddr
		hit.CreatedAt = time.Now().UTC()
		hit.Host = r.Host

		if !silent {
			zlog.Printf("buffering %s%s", r.Host, r.URL)
		}

		reqBuffer <- hit
		w.WriteHeader(http.StatusNoContent)
	})
}

func checkBackend(bufClient *http.Client, url string, isDown *zsync.AtomicInt) {
	var (
		setTo = int32(1)
		st    = ""
	)
	resp, err := bufClient.Get(url)
	if err == nil {
		resp.Body.Close()
		st = resp.Status
		if resp.StatusCode < 300 {
			setTo = 0
		}
	}

	v := isDown.Value()
	if v != setTo {
		if setTo == 0 {
			zlog.Printf("status of %q changed to UP: %s", url, st)
		} else {
			zlog.Printf("status of %q changed to DOWN: %s (err: %v)", url, st, err)
		}
		isDown.Set(setTo)
	}

	if setTo == 1 && time.Now().Second()%10 == 0 {
		zlog.Printf("backend %q is still DOWN", url)
	}
}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
	"zgo.at/goatcounter/handlers"
	"zgo.at/zdb"
	"zgo.at/zstd/zsync"
)

// TODO: -count=2 doesn't work as handlers/api.go has:
//   bufferKeyOnce = sync.Once{}
func TestBuffer(t *testing.T) {
	t.Skip() // TODO: flaky
	cfg.Reset()
	handlers.Reset()

	ctx, dbc, clean := tmpdb(t)
	defer clean()

	run(t, 0, []string{"buffer", "-generate-key", "-db", dbc})

	var key string
	err := zdb.Get(ctx, &key,
		`select value from store where key='buffer-secret'`)
	if err != nil {
		t.Fatal(err)
	}
	if key == "" {
		t.Fatal("key is empty")
	}

	err = os.Setenv("GOATCOUNTER_BUFFER_SECRET", key)
	if err != nil {
		t.Fatal(err)
	}

	i := zsync.NewAtomicInt(0)
	handle := handlers.NewBackend(zdb.MustGetDB(ctx), nil)
	goatcounter.Memstore.TestInit(zdb.MustGetDB(ctx))

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ii := i.Add(1)
		if r.URL.Path == "/status" {
			if ii < 5 {
				w.WriteHeader(500)
			}
			return
		}

		if ii < 5 {
			t.Fatalf("sent while down; ii: %d; URL: %s", ii, r.URL)
			w.WriteHeader(500)
			return
		}

		handle.ServeHTTP(w, r)
	}))

	_, site := gctest.Site(ctx, t, goatcounter.Site{})

	zdb.Dump(ctx, os.Stdout, `select * from sites`)

	errCh := make(chan error)
	go func() {
		time.Sleep(100 * time.Millisecond)
		r, _ := http.NewRequest("GET", "http://localhost:8082/count?p=/xxx", nil)
		r.Host = site.Code + ".localhost"
		resp, err := http.DefaultClient.Do(r)
		errCh <- err
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	// TODO: random -listen
	run(t, 0, []string{"buffer", "-backend", s.URL, "-test-hook-do-not-use=2"})

	cron.PersistAndStat(ctx)
	var out int
	err = zdb.Get(ctx, &out, `select count(*) from hits`)
	if err != nil {
		t.Fatal(err)
	}

	want := 1
	if out != want {
		t.Fatalf("\nout:  %d\nwant: %d", out, want)
	}

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

// l, err := net.Listen("tcp", "127.0.0.1:0")
// if err != nil {
// 	if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
// 		panic(fmt.Sprintf("httptest: failed to listen on a port: %v", err))
// 	}
// }
// return l

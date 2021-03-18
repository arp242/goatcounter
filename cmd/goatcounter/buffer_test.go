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
	"zgo.at/goatcounter/cron"
	"zgo.at/goatcounter/gctest"
	"zgo.at/goatcounter/handlers"
	"zgo.at/zdb"
	"zgo.at/zstd/zsync"
)

func TestBuffer(t *testing.T) {
	t.Skip() // TODO: rewrite this.

	exit, _, out, ctx, dbc := startTest(t)

	runCmd(t, exit, "buffer", "-generate-key", "-db="+dbc)
	wantExit(t, exit, out, 0)

	var key string
	{
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
	}

	i := zsync.NewAtomicInt(0)
	handle := handlers.NewBackend(zdb.MustGetDB(ctx), nil, false, false, "")
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

	ctx = gctest.Site(ctx, t, nil, nil)
	errCh := make(chan error)
	go func() {
		time.Sleep(100 * time.Millisecond)
		r, _ := http.NewRequest("GET", "http://localhost:8082/count?p=/xxx", nil)
		r.Host = goatcounter.MustGetSite(ctx).Code + ".localhost"
		resp, err := http.DefaultClient.Do(r)
		errCh <- err
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	go runCmdStop(t, exit, ready, stop, "buffer", "-backend="+s.URL)
	<-ready
	cron.PersistAndStat(ctx)

	{
		var got int
		err := zdb.Get(ctx, &got, `select count(*) from hits`)
		if err != nil {
			t.Fatal(err)
		}

		want := 1
		if got != want {
			t.Errorf("\ngot:  %d\nwant: %d\nstdout: %s", got, want, out)
		}

		<-stop
		mainDone.Wait()
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

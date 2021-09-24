// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/goatcounter/v2/handlers"
	"zgo.at/zdb"
	"zgo.at/zstd/zsync"
)

func TestBuffer(t *testing.T) {
	exit, _, out, ctx, dbc := startTest(t)
	//handlers.ResetBufferKey()
	bufCheckBackendTime = 200 * time.Millisecond
	bufSendTime = 200 * time.Millisecond

	// Get buffer key.
	var key string
	{
		runCmd(t, exit, "buffer", "-generate-key", "-db="+dbc)
		wantExit(t, exit, out, 0)

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

	// Set up a GoatCounter server
	var backend string
	{
		i := zsync.NewAtomicInt(0)
		handle := handlers.NewBackend(zdb.MustGetDB(ctx), nil, false, false, "", 10)
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

			*r = *r.WithContext(ctx)
			handle.ServeHTTP(w, r)
		}))
		backend = s.URL
	}

	ctx = gctest.Site(ctx, t, nil, nil)
	errCh := make(chan error, 16)
	send := func() {
		time.Sleep(100 * time.Millisecond)
		r, _ := http.NewRequest("GET", "http://localhost:8082/count?p=/xxx", nil)
		r.Host = goatcounter.MustGetSite(ctx).Code + "." + goatcounter.Config(ctx).Domain
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusNoContent {
			errCh <- fmt.Errorf("%d %s: %s", resp.StatusCode, resp.Status, b)
			return
		}
		if len(b) > 0 {
			t.Log(string(b))
		}
	}

	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	go runCmdStop(t, exit, ready, stop, "buffer", "-backend="+backend)
	<-ready

	var sendWg sync.WaitGroup
	sendWg.Add(8)
	go func() {
		for i := 0; i <= 7; i++ {
			send()
			sendWg.Done()
		}
	}()
	sendWg.Wait()

	for len(errCh) > 0 {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(bufSendTime * 2)
	cron.PersistAndStat(ctx)

	{
		var got int
		err := zdb.Get(ctx, &got, `select count(*) from hits`)
		if err != nil {
			t.Fatal(err)
		}

		want := 8
		if got != want {
			t.Errorf("\ngot:  %d\nwant: %d\nstdout: %s", got, want, out)
		}

		stop <- struct{}{}
		mainDone.Wait()
	}
}

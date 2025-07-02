package main

import (
	"io"
	"net/http"
	"testing"
)

func TestServe(t *testing.T) {
	exit, _, _, _, dbc := startTest(t)

	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	go runCmdStop(t, exit, ready, stop, "serve",
		"-db="+dbc,
		"-debug=all",
		"-listen=localhost:31874",
		"-tls=http")
	<-ready

	resp, err := http.Get("http://localhost:31874/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("status %d: %s", resp.StatusCode, b)
	}
	if len(b) < 100 {
		t.Errorf("%s", b)
	}

	stop <- struct{}{}
	mainDone.Wait()
}

func TestSaas(t *testing.T) {
	exit, _, _, _, dbc := startTest(t)

	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	go func() {
		runCmdStop(t, exit, ready, stop, "saas",
			"-db="+dbc,
			"-debug=all",
			"-domain=goatcounter.com,a.a",
			"-listen=localhost:31874",
			"-tls=http")
	}()
	<-ready

	resp, err := http.Get("http://localhost:31874/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Errorf("status %d: %s", resp.StatusCode, b)
	}
	if len(b) < 100 {
		t.Errorf("%s", b)
	}

	stop <- struct{}{}
	mainDone.Wait()
}

package main

import (
	"strings"
	"testing"
	"time"

	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
)

func TestMonitorOnce(t *testing.T) {
	t.Skip() // TODO: flaky test.

	exit, _, out, ctx, dbc := startTest(t)

	t.Run("no pageviews", func(t *testing.T) {
		runCmd(t, exit, "monitor",
			"-db="+dbc,
			"-once",
			"-debug=all")
		wantExit(t, exit, out, 1)
		if !strings.Contains(out.String(), "no hits in last") {
			t.Error(out.String())
		}
	})

	t.Run("with pageviews", func(t *testing.T) {
		gctest.StoreHits(ctx, t, false, goatcounter.Hit{})

		runCmd(t, exit, "monitor",
			"-db="+dbc,
			"-once",
			"-debug=all")
		wantExit(t, exit, out, 0)
		if !strings.Contains(out.String(), "1 hits") {
			t.Error(out.String())
		}
	})
}

func TestMonitorLoop(t *testing.T) {
	t.Skip() // TODO: flaky test.

	exit, _, out, ctx, dbc := startTest(t)

	gctest.StoreHits(ctx, t, false, goatcounter.Hit{})

	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	go runCmdStop(t, exit, ready, stop, "monitor",
		"-db="+dbc,
		"-period=1",
		"-debug=all")
	<-ready

	time.Sleep(2 * time.Second)
	stop <- struct{}{}
	mainDone.Wait()

	if !strings.Contains(out.String(), "no hits in last") || !strings.Contains(out.String(), "1 hits") {
		t.Error(out.String())
	}
}

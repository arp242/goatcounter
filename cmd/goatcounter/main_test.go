package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"zgo.at/blackmail"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/cron"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zli"
)

var pgSQL = false

// Make sure usage doesn't contain tabs, as that will mess up formatting in
// terminals.
func TestUsageTabs(t *testing.T) {
	for k, v := range usage {
		if strings.Contains(v, "\t") {
			t.Errorf("%q contains tabs", k)
		}
	}
}

func startTest(t *testing.T) (
	exit *zli.TestExit, in *bytes.Buffer, out *bytes.Buffer,
	ctx context.Context, dbc string,
) {
	t.Helper()

	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)

	goatcounter.Memstore.Reset()

	ctx = gctest.DBFile(t)

	exit, in, out = zli.Test(t)
	return exit, in, out, ctx, os.Getenv("GCTEST_CONNECT")
}

func runCmdStop(t *testing.T, exit *zli.TestExit, ready chan<- struct{}, stop chan struct{}, cmd string, args ...string) {
	defer exit.Recover()
	defer cron.Stop()
	cmdMain(zli.NewFlags(append([]string{"goatcounter", cmd}, args...)), ready, stop)
}

func runCmd(t *testing.T, exit *zli.TestExit, cmd string, args ...string) {
	ready := make(chan struct{}, 1)
	stop := make(chan struct{})
	runCmdStop(t, exit, ready, stop, cmd, args...)
	<-ready
}

func wantExit(t *testing.T, exit *zli.TestExit, out *bytes.Buffer, want int) {
	t.Helper()
	if int(*exit) != want {
		t.Errorf("wrong exit: %d; want: %d\n%s", *exit, want, out.String())
	}
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"zgo.at/blackmail"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zli"
	"zgo.at/zlog"
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
	ctx context.Context, dbc string, clean func(),
) {
	t.Helper()

	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)
	zlog.Config.Outputs = []zlog.OutputFunc{
		func(l zlog.Log) {
			out := zli.Stdout
			if l.Level == zlog.LevelErr {
				out = zli.Stderr
			}
			fmt.Fprintln(out, zlog.Config.Format(l))
		},
	}

	goatcounter.Memstore.Reset()

	ctx, dbClean := gctest.DBFile(t)

	exit, in, out, zliClean := zli.Test()
	return exit, in, out, ctx, os.Getenv("GCTEST_CONNECT"), func() {
		zliClean()
		dbClean()
	}
}

func runCmdStop(t *testing.T, exit *zli.TestExit, ready chan<- struct{}, stop chan struct{}, cmd string, args ...string) {
	defer exit.Recover()
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

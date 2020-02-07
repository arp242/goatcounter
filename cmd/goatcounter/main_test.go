// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"zgo.at/zlog"
)

// Make sure usage doesn't contain tabs, as that will mess up formatting in
// terminals.
func TestUsageTabs(t *testing.T) {
	for k, v := range usage {
		if strings.Contains(v, "\t") {
			t.Errorf("%q contains tabs", k)
		}
	}
}

func run(t *testing.T, killswitch string, args []string) {
	cwd, _ := os.Getwd()
	err := os.Chdir("../../")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	tmpdir, err := ioutil.TempDir("", "goatcounter")
	if err != nil {
		os.Chdir(cwd)
		t.Fatal(err)
	}
	tmpdb := "sqlite://" + tmpdir + "/goatcounter.sqlite3"
	defer os.RemoveAll(tmpdir)

	// Reset flags in case of -count 2
	CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	args = append(args, []string{"-db", tmpdb}...)
	os.Args = args

	// Swap out std/stderr.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout = w
	stderr = w
	zlog.Config.Outputs = []zlog.OutputFunc{
		func(l zlog.Log) {
			out := stdout
			if l.Level == zlog.LevelErr {
				out = stderr
			}
			fmt.Fprintln(out, zlog.Config.Format(l))
		},
	}

	// Kill when we see a string.
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			l := scanner.Text()
			fmt.Println("std: ", l)
			if strings.Contains(l, killswitch) {
				fmt.Println("kill", syscall.Getpid())
				time.Sleep(100 * time.Millisecond)
				stop()
			}
		}
	}()
	go func() {
		time.Sleep(10 * time.Second)
		stop()
	}()

	main()
}

func stop() {
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

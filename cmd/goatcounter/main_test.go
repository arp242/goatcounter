// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"bufio"
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"zgo.at/ztest"
)

func TestMain(t *testing.T) {
	// Just ensure the app can start with the default settings, creating a new
	// DB file.
	cwd, _ := os.Getwd()
	err := os.Chdir("../../")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	tmpdir, err := ioutil.TempDir("", "goatcounter")
	if err != nil {
		t.Fatal(err)
	}
	tmpdb := tmpdir + "/goatcounter.sqlite3"
	defer os.RemoveAll(tmpdir)

	// Reset flags in case of -count 2
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{"goatcounter",
		"-prod", "-smtp", "dummy",
		"-dbconnect", tmpdb,
		"-listen", "localhost:31874"}

	out, reset := ztest.ReplaceStdStreams()
	defer reset()
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "listening on") {
				time.Sleep(100 * time.Millisecond)
				syscall.Kill(syscall.Getpid(), syscall.SIGINT)
			}
		}
	}()

	main()
}

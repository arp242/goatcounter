// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"flag"
	"io/ioutil"
	"os"
	"syscall"
	"testing"
	"time"
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
		"-dbconnect", tmpdb,
		"-listen", "localhost:31874"}

	go func() {
		time.Sleep(3000 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	main()
}

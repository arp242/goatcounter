package main

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestMain(t *testing.T) {
	// Just ensure the app can start with the default settings, creating a new
	// DB file.
	err := os.Chdir("../../")
	if err != nil {
		t.Fatal(err)
	}

	tmpdir, err := ioutil.TempDir("", "goatcounter")
	if err != nil {
		t.Fatal(err)
	}
	tmpdb := tmpdir + "/goatcounter.sqlite3"
	defer os.RemoveAll(tmpdir)

	os.Args = []string{"goatcounter",
		"-dbconnect", tmpdb,
		"-listen", "localhost:31874"}

	go main()
	time.Sleep(500 * time.Millisecond)
}

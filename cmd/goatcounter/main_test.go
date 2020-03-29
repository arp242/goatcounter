// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"zgo.at/goatcounter/cfg"
	_ "zgo.at/goatcounter/gctest" // Set cfg.PgSQL
	"zgo.at/zdb"
	"zgo.at/zhttp"
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

func tmpdb(t *testing.T) (context.Context, string, func()) {
	t.Helper()

	var clean func()
	defer func() {
		r := recover()
		if r != nil {
			clean()
			panic(r)
		}
	}()

	dbname := "goatcounter_" + zhttp.Secret()
	var tmp string
	if cfg.PgSQL {
		out, err := exec.Command("createdb", dbname).CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s → %s", err, out))
		}
		clean = func() {
			out, err := exec.Command("dropdb", dbname).CombinedOutput()
			if err != nil {
				panic(fmt.Sprintf("%s → %s", err, out))
			}
		}

		out, err = exec.Command("psql", dbname, "-c", `\i ../../db/schema.pgsql`).CombinedOutput()
		if err != nil {
			panic(fmt.Sprintf("%s → %s", err, out))
		}

		tmp = "postgresql://dbname=" + dbname + " sslmode=disable password=x"
	} else {
		dir, err := ioutil.TempDir("", "goatcounter")
		if err != nil {
			t.Fatal(err)
		}
		clean = func() {
			os.RemoveAll(dir)
		}

		tmp = "sqlite://" + dir + "/goatcounter.sqlite3"
	}

	db, err := connectDB(tmp, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	return zdb.With(context.Background(), db), tmp, func() {
		db.Close()
		clean()
	}
}

func run(t *testing.T, killswitch string, args []string) ([]string, int) {
	cwd, _ := os.Getwd()
	err := os.Chdir("../../")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Reset flags in case of -count 2
	CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = append([]string{"goatcounter"}, args...)

	// Swap out stdout/stderr.
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

	// Record output, and kill when we see a string.
	var output []string
	wait := make(chan bool)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			l := scanner.Text()
			output = append(output, l)
			if killswitch != "" && strings.Contains(l, killswitch) {
				fmt.Println("kill", syscall.Getpid())
				time.Sleep(100 * time.Millisecond)
				stop()
			}
		}
		wait <- true
	}()

	// Safety.
	go func() {
		time.Sleep(20 * time.Second)
		wait <- false
	}()

	// Return exit code.
	var code int
	exit = func(c int) { code = c }

	main()

	w.Close()
	if !<-wait {
		stop()
		code = 99
		t.Fatal("test took longer than 20s")
	}
	return output, code
}

func stop() {
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

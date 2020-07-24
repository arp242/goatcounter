// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"zgo.at/blackmail"
	"zgo.at/goatcounter/cfg"
	_ "zgo.at/goatcounter/gctest" // Set cfg.PgSQL
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zcrypto"
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

	dbname := "goatcounter_" + zcrypto.Secret64()
	var tmp string
	if cfg.PgSQL {
		// TODO: don't rely on shell commands if possible, as it's quite slow.
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

	db, err := connectDB(tmp, []string{"all"}, true)
	if err != nil {
		t.Fatal(err)
	}

	return zdb.With(context.Background(), db), tmp, func() {
		db.Close()
		clean()
	}
}

func run(t *testing.T, wantCode int, args []string) {
	cwd, _ := os.Getwd()
	err := os.Chdir("../../")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Reset flags in case of -count 2
	CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = append([]string{"goatcounter"}, args...)
	blackmail.DefaultMailer = blackmail.NewMailer(blackmail.ConnectWriter)

	zlog.Config.Outputs = []zlog.OutputFunc{
		func(l zlog.Log) {
			out := stdout
			if l.Level == zlog.LevelErr {
				out = stderr
			}
			fmt.Fprintln(out, zlog.Config.Format(l))
		},
	}

	// Return exit code.
	var code int
	exit = func(c int) { code = c }

	main()

	if code != wantCode {
		t.Fatalf("exit code %d; want %d", code, wantCode)
	}
}

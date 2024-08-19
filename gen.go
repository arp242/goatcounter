// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

//go:build go_run_only

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"zgo.at/errors"
	"zgo.at/zstd/zio"
	"zgo.at/zstd/zruntime"
)

func main() {
	if _, ok := os.LookupEnv("CI"); ok {
		return
	}

	for _, f := range []func() error{kommentaar} {
		err := f()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", zruntime.FuncName(f), err)
		}
	}
}

// TODO: would be better to just generate this on the first request, but it
// takes about 10s now so that's a bit too slow :-/
func kommentaar() error {
	if !zio.ChangedFrom("./handlers/api.go", "./tpl/api.json") &&
		!zio.ChangedFrom("./kommentaar.conf", "./tpl/api.json") {
		return nil
	}

	commands := map[string][]string{
		"tpl/api.json": {"-config", "../kommentaar.conf", "-output", "openapi2-jsonindent", "."},
		"tpl/api.html": {"-config", "../kommentaar.conf", "-output", "html", "."},
	}

	for file, args := range commands {
		stderr := new(bytes.Buffer)
		cmd := exec.Command("kommentaar", args...)
		cmd.Dir = "./handlers"
		cmd.Stderr = stderr

		fmt.Println("running", cmd.Args)
		out, err := cmd.Output()
		if err != nil {
			out = stderr.Bytes()
			return errors.Errorf("running kommentaar: %s\n%s", err, out)
		}

		err = os.WriteFile(file, out, 0666)
		if err != nil {
			return errors.Errorf("kommentaar: %s\n%s", err)
		}
	}
	return nil
}

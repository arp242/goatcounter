// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// +build go_run_only

package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"zgo.at/errors"
	"zgo.at/zstd/zio"
	"zgo.at/zstd/zruntime"
)

func main() {
	if _, ok := os.LookupEnv("CI"); ok {
		return
	}

	for _, f := range []func() error{markdown, kommentaar} {
		err := f()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", zruntime.FuncName(f), err)
		}
	}
}

var (
	reHeaders = regexp.MustCompile(`<h([2-6]) id="(.*?)">(.*?)<\/h[2-6]>`)
	reTpl     = regexp.MustCompile(`(?:<p>({{)|(}})</p>)`)

	// {{template "%%top.gohtml" .}}
	reUnderscore = regexp.MustCompile(`template "%%`)
)

func kommentaar() error {
	if !zio.ChangedFrom("./handlers/api.go", "./tpl/api.json") {
		return nil
	}

	commands := map[string][]string{
		"tpl/api.json": {"-config", "../kommentaar.conf", "-output", "openapi2-jsonindent", "."},
		"tpl/api.html": {"-config", "../kommentaar.conf", "-output", "html", "."},
	}

	for file, args := range commands {
		cmd := exec.Command("kommentaar", args...)
		cmd.Dir = "./handlers"

		out, err := cmd.Output()
		if err != nil {
			return errors.Errorf("running kommentaar: %s\n%s", err, out)
		}

		err = os.WriteFile(file, out, 0666)
		if err != nil {
			return errors.Errorf("kommentaar: %s\n%s", err)
		}
	}
	return nil
}

// TODO: implement something to generate and cache markdown on requests, so we
// can get rid of the generate step.
func markdown() error {
	ls, err := os.ReadDir("./tpl")
	if err != nil {
		return err
	}

	for _, f := range ls {
		src := "tpl/" + f.Name()
		if !strings.HasSuffix(src, ".markdown") {
			continue
		}
		dst := src[:len(src)-9] + ".gohtml"

		if !zio.ChangedFrom(src, dst) {
			continue
		}

		out, err := exec.Command("kramdown", "--smart-quotes", "39,39,34,34", src).CombinedOutput()
		if err != nil {
			return errors.Errorf("running kramdown: %s\n%s", err, out)
		}

		dest, err := os.Create(dst)
		if err != nil {
			return err
		}
		line := strings.Repeat("*", 72)
		_, err = dest.Write([]byte(fmt.Sprintf("{{/*%s\n * This file was generated from %s. DO NOT EDIT.\n%[1]s*/}}\n\n",
			line, src)))
		if err != nil {
			return err
		}

		out = reHeaders.ReplaceAll(out, []byte(`<h$1 id="$2">$3 <a href="#$2"></a></h$1>`))
		out = reTpl.ReplaceAll(out, []byte("$1$2"))
		out = reUnderscore.ReplaceAll(out, []byte(`template "_`))

		_, err = dest.Write(out)
		if err != nil {
			return err
		}
		err = dest.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

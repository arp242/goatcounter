// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// +build go_run_only

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"zgo.at/errors"
	"zgo.at/zpack"
)

func main() {
	if _, ok := os.LookupEnv("CI"); !ok {
		err := markdown()
		if err != nil {
			fmt.Fprintf(os.Stderr, "non-fatal error: unable to generate markdown files: %s\n", err)
		}

		err = kommentaar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "non-fatal error: unable to generate kommentaar files: %s\n", err)
		}
	}

	err := zpack.Pack(map[string]map[string]string{
		"./pack/pack.go": {
			"Public":           "./public",
			"Templates":        "./tpl",
			"SchemaSQLite":     "./db/schema.sql",
			"SchemaPgSQL":      "./db/schema.pgsql",
			"MigrationsSQLite": "./db/migrate/sqlite",
			"MigrationsPgSQL":  "./db/migrate/pgsql",
		},
	}, "/.keep", "public/fonts/LICENSE", ".markdown", "/index.html")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Don't need to commit this.
	if _, err := os.Stat("./GeoLite2-Country.mmdb"); err == nil {
		err := zpack.Pack(map[string]map[string]string{
			"./pack/geodb.go": {
				"GeoDB": "./GeoLite2-Country.mmdb",
			},
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
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
	commands := map[string][]string{
		"docs/api.yaml": {"-config", "./kommentaar.conf", "-output", "openapi2-yaml", "./handlers"},
		"docs/api.json": {"-config", "./kommentaar.conf", "-output", "openapi2-jsonindent", "./handlers"},
		"docs/api.html": {"-config", "./kommentaar.conf", "-output", "html", "./handlers"},
	}

	for file, args := range commands {
		out, err := exec.Command("kommentaar", args...).CombinedOutput()
		if err != nil {
			return errors.Errorf("running kommentaar: %s\n%s", err, out)
		}

		err = ioutil.WriteFile(file, out, 0666)
		if err != nil {
			return errors.Errorf("kommentaar: %s\n%s", err)
		}
	}
	return nil
}

// Don't really need to generate Markdown on requests, and don't want to
// implement caching; so just go generate it.
func markdown() error {
	ls, err := ioutil.ReadDir("./tpl")
	if err != nil {
		return err
	}

	for _, f := range ls {
		src := "tpl/" + f.Name()
		if !strings.HasSuffix(src, ".markdown") {
			continue
		}
		dst := src[:len(src)-9] + ".gohtml"

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

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// +build go_run_only

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"zgo.at/zpack"
)

func main() {
	if _, ok := os.LookupEnv("CI"); !ok {
		err := markdown()
		if err != nil {
			fmt.Fprintf(os.Stderr, "non-fatal error: unable to generate markdown files: %s\n", err)
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
	reHeaders = regexp.MustCompile(`<h(\d) id="(.*?)">(.*?)<\/h\d>`)
	reTpl     = regexp.MustCompile(`(?:<p>({{)|(}})</p>)`)
)

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
			return fmt.Errorf("running kramdown: %s\n%s", err, out)
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

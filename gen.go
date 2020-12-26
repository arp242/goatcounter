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
	"text/template"

	"zgo.at/errors"
	"zgo.at/zlog"
	"zgo.at/zpack"
	"zgo.at/zstd/zioutil"
)

func main() {
	l := zlog.Module("gen")

	if _, ok := os.LookupEnv("CI"); !ok {
		err := markdown()
		if err != nil {
			fmt.Fprintf(os.Stderr, "non-fatal error: unable to generate markdown files: %s\n", err)
		}
		l = l.Since("markdown")

		err = kommentaar()
		if err != nil {
			fmt.Fprintf(os.Stderr, "non-fatal error: unable to generate kommentaar files: %s\n", err)
		}
		l = l.Since("kommentaar")

		err = schema()
		if err != nil {
			fmt.Fprintf(os.Stderr, "non-fatal error: unable to generate DB schema files: %s\n", err)
		}
		l = l.Since("schema")
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

	l = l.Since("pack")

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
	l.FieldsSince().Print("done")
}

func schema() error {
	tpl, err := ioutil.ReadFile("./db/schema.gotxt")
	if err != nil {
		return err
	}

	var pgsql bool
	t := template.Must(template.New("").Funcs(template.FuncMap{
		"sqlite": func(s string) string {
			if pgsql {
				return ""
			}
			return s
		},
		"psql": func(s string) string {
			if pgsql {
				return s
			}
			return ""
		},
		"auto_increment": func() string {
			if pgsql {
				return "serial         primary key"
			}
			return "integer        primary key autoincrement"
		},
		"jsonb": func() string {
			if pgsql {
				return "jsonb    "
			}
			return "varchar  "
		},
		"blob": func() string {
			if pgsql {
				return "bytea   "
			}
			return "blob    "
		},
		"check_timestamp": func(col string) string {
			if pgsql {
				return ""
			}
			return "check(" + col + " = strftime('%Y-%m-%d %H:%M:%S', " + col + "))"
		},
		"check_date": func(col string) string {
			if pgsql {
				return ""
			}
			return "check(" + col + " = strftime('%Y-%m-%d', " + col + "))"
		},
		"cluster": func(tbl, idx string) string {
			if pgsql {
				return `cluster ` + tbl + ` using "` + idx + `";`
			}
			return ""
		},
		"replica": func(tbl, idx string) string {
			if pgsql {
				return `alter table ` + tbl + ` replica identity using index "` + idx + `";`
			}
			return ""
		},
	}).Parse(string(tpl)))

	{
		fp, err := os.Create("./db/schema.sql")
		if err != nil {
			return (err)
		}

		err = t.Execute(fp, nil)
		if err != nil {
			return err
		}

		err = fp.Close()
		if err != nil {
			return (err)
		}
	}

	{
		pgsql = true
		fp, err := os.Create("./db/schema.pgsql")
		if err != nil {
			return (err)
		}
		err = t.Execute(fp, nil)

		err = fp.Close()
		if err != nil {
			return (err)
		}
	}

	return nil
}

var (
	reHeaders = regexp.MustCompile(`<h([2-6]) id="(.*?)">(.*?)<\/h[2-6]>`)
	reTpl     = regexp.MustCompile(`(?:<p>({{)|(}})</p>)`)

	// {{template "%%top.gohtml" .}}
	reUnderscore = regexp.MustCompile(`template "%%`)
)

func kommentaar() error {
	if !zioutil.ChangedFrom("./handlers/api.go", "./tpl/api.json") {
		return nil
	}

	commands := map[string][]string{
		"tpl/api.json": {"-config", "../kommentaar.conf", "-output", "openapi2-jsonindent", "."},
		"tpl/api.html": {"-config", "../kommentaar.conf", "-output", "html", "."},
	}

	for file, args := range commands {
		cmd := exec.Command("kommentaar", args...)
		cmd.Dir = "./handlers"

		out, err := cmd.CombinedOutput()
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

		if !zioutil.ChangedFrom(src, dst) {
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

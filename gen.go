// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// +build go_run_only

package main

import (
	"fmt"
	"os"

	"zgo.at/zpack"
)

func main() {
	err := zpack.Pack(map[string]map[string]string{
		"./pack/pack.go": map[string]string{
			"Public":           "./public",
			"Templates":        "./tpl",
			"SchemaSQLite":     "./db/schema.sql",
			"SchemaPgSQL":      "./db/schema.pgsql",
			"MigrationsSQLite": "./db/migrate/sqlite",
			"MigrationsPgSQL":  "./db/migrate/pgsql",
		},
	}, "/.keep", "public/fonts/LICENSE")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Don't need to commit this.
	if _, err := os.Stat("./GeoLite2-Country.mmdb"); err == nil {
		err := zpack.Pack(map[string]map[string]string{
			"./pack/geodb.go": map[string]string{
				"GeoDB": "./GeoLite2-Country.mmdb",
			},
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

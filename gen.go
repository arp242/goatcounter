// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

// +build go_run_only

package main

import (
	"fmt"
	"os"

	"zgo.at/zpack"
)

func main() {
	err := zpack.Pack(map[string]map[string]string{
		"./db/pack.go": map[string]string{
			"Schema":           "./db/schema.sql",
			"MigrationsPgSQL":  "./db/migrate/pgsql",
			"MigrationsSQLite": "./db/migrate/sqlite",
		},
		"./handlers/pack.go": map[string]string{
			"packPublic": "./public",
			"packTpl":    "./tpl",
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

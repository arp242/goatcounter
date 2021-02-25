// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"testing"
	"testing/fstest"
)

func TestEmbed(t *testing.T) {
	err := fstest.TestFS(DB, "db/schema-sqlite.sql", "db/migrate/2020-08-28-1-paths-tables-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}

	err = fstest.TestFS(DB, "db/goatcounter.sqlite3", "db/migrate/gomig/gomig.go")
	if err == nil {
		t.Fatal("db/goatcounter.sqlite3 in embeded files")
	}
}

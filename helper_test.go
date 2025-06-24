package goatcounter

import (
	"testing"
	"testing/fstest"
)

func TestEmbed(t *testing.T) {
	err := fstest.TestFS(DB, "db/schema.gotxt", "db/migrate/2022-10-17-1-campaigns.gotxt")
	if err != nil {
		t.Fatal(err)
	}

	err = fstest.TestFS(DB, "db/goatcounter.sqlite3", "db/migrate/gomig/gomig.go")
	if err == nil {
		t.Fatal("db/goatcounter.sqlite3 in embeded files")
	}
}

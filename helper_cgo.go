//go:build cgo

package goatcounter

import "github.com/mattn/go-sqlite3"

func init() {
	sqlite3.SQLiteTimestampFormats = []string{"2006-01-02 15:04:05", "2006-01-02"}
}

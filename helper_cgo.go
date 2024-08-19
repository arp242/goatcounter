// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

//go:build cgo

package goatcounter

import "github.com/mattn/go-sqlite3"

func init() {
	sqlite3.SQLiteTimestampFormats = []string{"2006-01-02 15:04:05", "2006-01-02"}
}

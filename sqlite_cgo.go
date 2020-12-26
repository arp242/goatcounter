// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// +build cgo

package goatcounter

import "github.com/mattn/go-sqlite3"

// c.Exec() isn't defined in the cgo shim; can remove once
//   https://github.com/mattn/go-sqlite3/pull/894
// is merged
func sqliteCacheSize(c *sqlite3.SQLiteConn) error {
	_, err := c.Exec("PRAGMA cache_size = -20000", nil)
	return err
}

// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

// +build !cgo

package goatcounter

import "github.com/mattn/go-sqlite3"

func sqliteCacheSize(c *sqlite3.SQLiteConn) error { return nil }

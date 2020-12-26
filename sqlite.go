// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"math"

	"github.com/mattn/go-sqlite3"
)

var SQLiteHook = func(c *sqlite3.SQLiteConn) error {
	// Defauly is -2000, or 2M. Increase to 500M.
	//
	// This is the *maximum* memory the cache can use; in practice it will use a
	// lot less. It seems that setting this to more than ~20M has little effect,
	// but use 500M to have plenty of headway in case SQLite can use it.
	//
	// GC itself uses about ~35M to ~100M of memory, depending a lot on how busy
	// the site is, and even ~600M of maxiumum memory usage isn't all that much.
	//
	// Performance differences:
	//    With db.SetMaxOpenConns(1):
	//       -2000:  5762ms
	//      -20000:  4714ms
	//
	//    With db.SetMaxOpenConns(20):
	//       -2000:  3067ms
	//      -20000:  2532ms
	err := sqliteCacheSize(c)
	if err != nil {
		return err
	}

	return c.RegisterFunc("percent_diff", func(start, final int) float64 {
		if start == 0 {
			return math.Inf(0)
		}
		return float64(float64((final-start)/start) * 100)
	}, true)
}

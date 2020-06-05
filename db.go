// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"math"

	"github.com/mattn/go-sqlite3"
)

var SQLiteHook = func(c *sqlite3.SQLiteConn) error {
	//return c.RegisterFunc("percent_diff", func(start, final float64) float64 {
	return c.RegisterFunc("percent_diff", func(start, final int) float64 {
		if start == 0 {
			return math.Inf(0)
		}
		return float64(float64((final-start)/start) * 100)
	}, true)
}

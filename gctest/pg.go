// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// +build testpg

package gctest

import (
	"zgo.at/goatcounter/cfg"
)

func init() {
	cfg.PgSQL = true
}

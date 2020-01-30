// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// +build testpg

package goatcounter

import (
	"os/exec"

	"zgo.at/goatcounter/cfg"
)

func init() {
	cfg.PgSQL = true

	// Doing this on every test run doubles the running time.
	exec.Command("dropdb", "goatcounter_test").CombinedOutput()
	out, err := exec.Command("createdb", "goatcounter_test").CombinedOutput()
	if err != nil {
		panic(string(out))
	}
}

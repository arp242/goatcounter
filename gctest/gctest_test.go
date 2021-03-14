// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package gctest

import (
	"fmt"
	"testing"

	"zgo.at/zlog"
)

func TestDB(t *testing.T) {
	zlog.SetDebug("gctest")
	t.Run("", func(t *testing.T) {
		fmt.Println("Run 1")
		DB(t)
	})

	t.Run("", func(t *testing.T) {
		fmt.Println("\nRun 2")
		DB(t)
	})

	t.Run("", func(t *testing.T) {
		fmt.Println("\nRun 3")
		DB(t)
	})
}

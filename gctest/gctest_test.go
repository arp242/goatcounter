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
	fmt.Println("Run 1")
	_, clean := DB(t)
	clean()

	fmt.Println("\nRun 2")
	_, clean = DB(t)
	clean()

	fmt.Println("\nRun 3")
	_, clean = DB(t)
	clean()
}

// func BenchmarkTestDBDB(b *testing.B) {
// 	b.ReportAllocs()
// 	for n := 0; n < b.N; n++ {
// 		_, clean := DB(b)
// 		clean()
// 	}
// }

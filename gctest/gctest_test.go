// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

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

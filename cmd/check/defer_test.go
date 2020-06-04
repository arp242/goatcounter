// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDefer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Defer, "deferr")
}

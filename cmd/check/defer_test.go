// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"runtime"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDefer(t *testing.T) {
	// Doesn't run on Travis:
	//
	// analysistest.go:262: go tool not available: 'go env GOROOT' does not match runtime.GOROOT:
	//     	go env: C:\Users\travis\.gimme\versions\go1.13.12.windows.amd64
	//     	GOROOT: C:/Users/travis/.gimme/versions/go1.13.12.windows.amd64
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Defer, "deferr")
}

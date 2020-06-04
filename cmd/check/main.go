// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	// Vet checks.
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"

	// Additional checks in x/tools
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"

	// Staticcheck
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func main() {
	var checks = []*analysis.Analyzer{
		// All cmd/vet analyzers.
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		structtag.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,

		// Additional checks from x/tools
		atomicalign.Analyzer,
		deepequalerrors.Analyzer,
		ifaceassert.Analyzer,
		nilness.Analyzer,
		//shadow.Analyzer,
		sortslice.Analyzer,
		stringintconv.Analyzer,
		testinggoroutine.Analyzer,
	}

	// Most of staticcheck.
	for _, v := range simple.Analyzers {
		checks = append(checks, v)
	}
	for _, v := range staticcheck.Analyzers {
		checks = append(checks, v)
	}
	for k, v := range stylecheck.Analyzers {
		// - At least one file in a non-main package should have a package comment
		// - The comment should be of the form "Package x ..."
		if k == "ST1000" {
			continue
		}
		// The documentation of an exported function should start with
		// the function's name.
		if k == "ST1020" {
			continue
		}
		checks = append(checks, v)
	}

	// Our own stuff.
	checks = append(checks, Copyright, Defer)

	// Add -printf.funcs unless already given.
	var has bool
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-printf.funcs") {
			has = true
			break
		}
	}
	if !has {
		os.Args = append(
			[]string{os.Args[0], "-printf.funcs", "zgo.at/errors.Wrapf"},
			os.Args[1:]...)
	}

	multichecker.Main(checks...)
}

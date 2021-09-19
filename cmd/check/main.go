// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

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
	"honnef.co/go/tools/config"
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
		// composite.Analyzer,
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

	config.DefaultConfig.Initialisms = append(config.DefaultConfig.Initialisms, "ISO")

	// Most of staticcheck.
	for _, v := range simple.Analyzers {
		checks = append(checks, v.Analyzer)
	}
	for _, v := range staticcheck.Analyzers {
		if v.Analyzer.Name == "SA5008" {
			// Skip struct tags check, as our modified JSON adds a new ,readonly tag and this will error:
			//   unknown JSON option "readonly"
			// TODO: write a patch to staticcheck to allow adding fields or
			// something.
			continue
		}
		checks = append(checks, v.Analyzer)
	}
	for _, v := range stylecheck.Analyzers {
		// - At least one file in a non-main package should have a package comment
		// - The comment should be of the form "Package x ..."
		if v.Analyzer.Name == "ST1000" {
			continue
		}
		// The documentation of an exported function should start with
		// the function's name.
		if v.Analyzer.Name == "ST1020" {
			continue
		}
		// Skip for now due to bug in staticcheck in locations.go
		// TODO: lint:[..] directives don't seem to work. Actually,
		// staticcheck error codes are also ignored. Guess that's some frontend
		// it added on x/analysis?
		// TODO: send patch upstream.
		if v.Analyzer.Name == "ST1003" {
			continue
		}

		checks = append(checks, v.Analyzer)
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

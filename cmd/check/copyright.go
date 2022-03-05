// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"strings"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts"
)

var Copyright = &analysis.Analyzer{
	Name:     "copyright",
	Doc:      "Check that the first comment is a copyright notice.",
	Requires: []*analysis.Analyzer{facts.Generated},
	Run:      runCopyright,
}

func runCopyright(pass *analysis.Pass) (interface{}, error) {
	for _, f := range pass.Files {
		if code.IsGenerated(pass, f.Pos()) {
			continue
		}

		if len(f.Comments) == 0 {
			pass.Reportf(f.Pos(), "no copyright")
			continue
		}

		if !strings.Contains(f.Comments[0].Text(), "Copyright") {
			t := f.Comments[0].Text()
			if i := strings.IndexByte(t, '\n'); i > -1 {
				t = t[:i]
			}
			if len(t) > 30 {
				t = strings.TrimSpace(t[:30]) + "…"
			}
			pass.Reportf(f.Comments[0].Pos(), "first comment isn't copyright: %q", t)
		}
	}

	return nil, nil
}

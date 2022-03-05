// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Defer = &analysis.Analyzer{
	Name:     "defer",
	Doc:      "Check that function returns in defered statements are called.",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runDefer,
}

func runDefer(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.DeferStmt)(nil),
	}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		def := n.(*ast.DeferStmt)

		var ft *ast.FuncType
		switch c := def.Call.Fun.(type) {
		default:
			return
		case *ast.FuncLit: // defer func() { }()
			ft = c.Type
		case *ast.Ident: // defer f()
			// Obj is nil for builtins such as close(), copy(), etc. Since these
			// never return a function it's okay to just skip them.
			if c.Obj != nil {
				fd, ok := c.Obj.Decl.(*ast.FuncDecl)
				if !ok { // I think this should never happen?
					return
				}
				ft = fd.Type
			}
		}

		if ft != nil && returnsFunction(ft) {
			pass.Reportf(def.Call.Pos(), "defered return not called")
		}
	})

	return nil, nil
}

func returnsFunction(f *ast.FuncType) bool {
	if f.Results == nil {
		return false
	}
	for _, r := range f.Results.List {
		if _, ok := r.Type.(*ast.FuncType); ok {
			return true
		}
	}
	return false
}

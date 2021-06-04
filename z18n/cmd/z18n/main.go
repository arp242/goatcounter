package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"zgo.at/zli"
	"zgo.at/zstd/zstring"
	"zgo.at/ztpl/parse"
)

type entry struct {
	id, def string
	loc     []string
}

// z18n scan                         List all template strings
// z18n merge file.go                Merge with existing:
//                                     - Add new strings
//                                     - Modify locations of existing
//                                     - Comment out those that don't exist any more
// z18n convert file.go file.toml    Convert to/from
//
// Write this to Go files, e.g:
//
//    z18n.Messages = map[language.Tag]map[string]z18n.Msg{
//       language.Dutch: map[string]z18n.Msg{
//     	"": Msg{},
//       }
//    }
//
// Don't need to muck about with TOML, JSON, whatnot IMO.
//
// Can add "z18n convert" to convert from those formats, to send to translators
// etc.
//
// TODO: find context too.
func main() {
	found := findGo("./...") // TODO: flag

	for k, v := range findTpl("./tpl") { // TODO: flag
		e, ok := found[k]
		if ok {
			e.loc = append(e.loc, v.loc...)
		} else {
			e = v
		}

		found[k] = v
	}

	ord := make([]entry, 0, len(found))
	for _, f := range found {
		ord = append(ord, f)
	}
	sort.Slice(ord, func(i, j int) bool { return ord[i].id > ord[j].id })

	// TODO: print this:
	//   # tpl/dashboard.gohtml:6
	//   # tpl/dashboard.gohtml:7
	//   # tpl/dashboard.gohtml:8
	//   # tpl/dashboard.gohtml:9
	// as:
	//   # tpl/dashboard.gohtml:6-9
	for _, x := range ord {
		for _, l := range x.loc {
			fmt.Println("#", l)
		}
		fmt.Println(x.id, "=", x.def)
		fmt.Println()
	}
}

func findTpl(pattern string) map[string]entry {
	found := make(map[string]entry)
	err := filepath.WalkDir(pattern, func(path string, d fs.DirEntry, err error) error {
		zli.F(err)

		// TODO: add flag for specifying the extenions.
		if d.IsDir() || !zstring.HasSuffixes(d.Name(), ".gotxt", ".gohtml") {
			return nil
		}

		data, err := os.ReadFile(path)
		zli.F(err)

		tree, err := parse.Parse("", string(data), parse.ParseRelaxFunctions, "{{", "}}")
		zli.F(err)

		// fmt.Println("XXXXXX", path)
		// parse.PrintTree(os.Stdout, tree[""].Root)

		parse.Visit(tree[""].Root, func(n parse.Node, _ int) bool {
			nn, ok := n.(*parse.ActionNode)
			if !ok {
				return true
			}

			if nn.Pipe == nil || len(nn.Pipe.Cmds) == 0 || len(nn.Pipe.Cmds[0].Args) < 2 {
				return false
			}

			name := ""
			switch f := nn.Pipe.Cmds[0].Args[0].(type) {
			case *parse.IdentifierNode:
				name = f.Ident
			case *parse.FieldNode:
				name = strings.Join(f.Ident, ".")
			}
			if name != "t" && name != "T" { // TODO: add flag
				return false
			}

			idlit, ok := nn.Pipe.Cmds[0].Args[1].(*parse.StringNode)
			if !ok || idlit.Text == "" {
				idlit, ok = nn.Pipe.Cmds[0].Args[2].(*parse.StringNode)
				if !ok || idlit.Text == "" {
					return false
				}
			}
			id := idlit.Text
			id, def := zstring.Split2(id, "|")

			e := entry{id: id, def: def}
			f, ok := found[id]
			if ok {
				e.loc = f.loc
			}

			e.loc = append(e.loc, fmt.Sprintf("%s:%d", path, nn.Line))
			found[id] = e
			return false
		})

		return nil
	})
	zli.F(err)

	return found
}

func findGo(pattern string) map[string]entry {
	cwd, err := os.Getwd()
	zli.F(err)
	cwd += "/"

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedName | packages.NeedTypes,
	}, pattern)
	zli.F(err)

	found := make(map[string]entry)
	for _, p := range pkgs {
		if strings.Contains(p.PkgPath, "z18n") {
			continue
		}

		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				if len(c.Args) < 1 {
					return true
				}
				idlit, ok := c.Args[0].(*ast.BasicLit)
				if !ok || idlit.Kind != token.STRING {
					if len(c.Args) > 1 {
						idlit, ok = c.Args[1].(*ast.BasicLit)
					}
					if !ok || idlit.Kind != token.STRING {
						return true
					}
				}
				id := idlit.Value

				// Accept both "z18n.T" and just "T", in case you have a "var T = z18n.T" shortcut.
				//
				// TODO: would be better to resolve this.
				switch cc := c.Fun.(type) {
				default:
					return true
				case *ast.Ident:
					if cc.Name != "T" { // TODO: add flag
						return true
					}
				case *ast.SelectorExpr:
					if cc.Sel.Name != "z18n" {
						return true
					}
					ind, ok := cc.X.(*ast.Ident)
					if !ok || ind.Name != "T" {
						return true
					}
				}

				// TODO: if this id already exists, then check if it matches
				// what we found previously.
				// TODO: validate if arguments matches number of placeholders.

				b := new(bytes.Buffer)
				printer.Fprint(b, p.Fset, c)

				id = strings.Trim(id, "\"`")

				id, def := zstring.Split2(id, "|")
				e := entry{id: id, def: def}
				if f, ok := found[id]; ok {
					e.loc = f.loc
				}

				pos := p.Fset.Position(c.Pos())

				e.loc = append(e.loc, fmt.Sprintf("%s:%d", strings.TrimPrefix(pos.Filename, cwd), pos.Line))
				found[id] = e

				return true
			})
		}
	}

	return found
}

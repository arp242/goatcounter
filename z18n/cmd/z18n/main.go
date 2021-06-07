package main

import (
	"fmt"
	"os"

	"zgo.at/goatcounter/z18n"
	"zgo.at/goatcounter/z18n/finder"
	"zgo.at/zli"
)

const usage = `
z18n scans files for translatable strings.

https://github.com/zgoat/z18n

Commands:

    convert [in] [out]      Convert a file between formats.

    find [pattern..]        Find message strings

        -merge      Merge with existing file.

        -format     Output format:
                        go
                        toml
                        json
                        gettext

        -fun        Function names; default: "z18n.T", "z18n.Locale.T", "T"

        -tpl-ext    Extensions for templates; default is "gohtml" and "gotxt". Can
                    be given more than once.

        -tpl-fun    Template functions; default is "t" and ".T"
`

func main() {
	f := zli.NewFlags(os.Args)
	var (
		merge  = f.String("", "merge")
		format = f.String("toml", "format")
		fun    = f.StringList([]string{"z18n.T", "z18n.Locale.T", "T"}, "fun")
		tplExt = f.StringList([]string{"gohtml", "gotxt"}, "tpl-ext")
		tplFun = f.StringList([]string{".T", "t"}, "tpl-fun")
	)
	zli.F(f.Parse())

	switch f.ShiftCommand("convert", "find") {
	case zli.CommandAmbiguous, zli.CommandUnknown, zli.CommandNoneGiven:
		zli.Fatalf("cmd wrong")
	case "convert":
		if len(f.Args) != 2 {
			zli.Fatalf("len")
		}
		zli.F(convert(f.Args[0], f.Args[1]))
	case "find":
		dirs := []string{"./..."}
		if len(f.Args) > 1 {
			dirs = f.Args[1:]
		}
		found, err := find(dirs, fun.Strings(), tplFun.Strings(), tplExt.Strings())
		zli.F(err)

		f, ok := map[string]func() (string, error){
			"toml":    found.TOML,
			"json":    found.JSON,
			"go":      found.Go,
			"gettext": found.Gettext,
		}[format.String()]
		if !ok {
			zli.Fatalf("unknown format: %q", format)
		}

		// TODO: merge
		//
		// Read other file(s), and:
		//
		// 1. Add new entries
		// 2. Comment out entries that no longer exist.
		// 3. Update entries where the default has changed.
		//
		// We also need to read the "default messages" from a file (if any), so
		// we know if the base language has updated its text.
		_ = merge

		out, err := f()
		zli.F(err)
		fmt.Print(out)
	}
}

func convert(in, out string) error {
	// TODO
	z18n.ReadMessages("msg.toml")
	return nil
}

func find(dirs, fun, tplFun, tplExt []string) (finder.Entries, error) {
	found := make(finder.Entries)
	for _, d := range dirs {
		f, err := finder.Go(d, fun...)
		if err != nil {
			return nil, err
		}
		found.Merge(f)

		f, err = finder.Template(d, tplExt, tplFun...)
		if err != nil {
			return nil, err
		}
		found.Merge(f)
	}
	return found, nil
}

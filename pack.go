// +build generate

package main

import (
	"fmt"
	"os"

	"zgo.at/zhttp"
)

func main() {
	err := pack()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func pack() error {
	fp, err := os.Create("./handlers/pack.go")
	if err != nil {
		return err
	}
	var closeErr error
	defer func() { closeErr = fp.Close() }()

	err = zhttp.Header(fp, "handlers")
	if err != nil {
		return err
	}

	err = zhttp.PackDir(fp, "packPublic", "./public")
	if err != nil {
		return err
	}

	err = zhttp.PackDir(fp, "packTpl", "./tpl")
	if err != nil {
		return err
	}

	return closeErr
}

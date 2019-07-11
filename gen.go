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

	err = packDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func packDB() error {
	fp, err := os.Create("./db/pack.go")
	if err != nil {
		return err
	}
	var closeErr error
	defer func() { closeErr = fp.Close() }()

	err = zhttp.Header(fp, "db")
	if err != nil {
		return err
	}

	// DB schema.
	err = zhttp.PackFile(fp, "Schema", "db/schema.sql")
	if err != nil {
		return err
	}

	return closeErr
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

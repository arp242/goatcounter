// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zlog"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zvalidate"
)

const usageCreate = `
Create a new site and user.

Required flags:

  -domain      Domain to host e.g. "stats.example.com". The site will be
               available on this domain only, so "stats.example.com" won't be
               available on "localhost".

  -email       Your email address. Will be required to login.

Other flags:

  -password    Password to log in; will be asked interactively if omitted.

  -parent      Parent site; either as ID or domain.

  -db          Database connection: "sqlite://<file>" or "postgres://<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite://db/goatcounter.sqlite3?_busy_timeout=200&_journal_mode=wal&cache=shared

  -createdb    Create the database if it doesn't exist yet; only for SQLite.

  -debug       Modules to debug, comma-separated or 'all' for all modules.
`

func create() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()

	var (
		domain, email, parent, password string
		createdb                        bool
	)
	CommandLine.StringVar(&domain, "domain", "", "")
	CommandLine.StringVar(&email, "email", "", "")
	CommandLine.StringVar(&parent, "parent", "", "")
	CommandLine.StringVar(&password, "password", "", "")
	CommandLine.BoolVar(&createdb, "createdb", false, "")
	err := CommandLine.Parse(os.Args[2:])
	if err != nil {
		return 1, err
	}

	zlog.Config.SetDebug(*debug)
	cfg.Serve = true

	v := zvalidate.New()
	v.Required("-domain", domain)
	v.Required("-email", email)
	v.Domain("-domain", domain)
	v.Email("-email", email)
	if v.HasErrors() {
		return 1, v
	}

	if password == "" {
	getpw:
		fmt.Printf("Enter password for new user (will not echo): ")
		pwd, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return 3, err
		}
		if len(pwd) < 8 {
			fmt.Println("\nNeed at least 8 characters")
			goto getpw
		}

		fmt.Printf("\nConfirm: ")
		pwd2, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return 3, err
		}
		fmt.Println("")

		if !bytes.Equal(pwd, pwd2) {
			fmt.Println("Passwords did not match; try again.")
			goto getpw
		}

		password = string(pwd)
	}

	db, err := connectDB(*dbConnect, nil, createdb)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	var ps goatcounter.Site
	if parent != "" {
		ps, err = findParent(zdb.With(context.Background(), db), parent)
		if err != nil {
			return 1, err
		}
	}

	err = zdb.TX(zdb.With(context.Background(), db), func(ctx context.Context, tx zdb.DB) error {
		s := goatcounter.Site{
			Code:  "serve-" + zcrypto.Secret64(),
			Cname: &domain,
			Plan:  goatcounter.PlanBusinessPlus,
		}
		if ps.ID > 0 {
			s.Parent = &ps.ID
			s.Settings = ps.Settings
			s.Plan = goatcounter.PlanChild
		}
		err := s.Insert(ctx)
		if err != nil {
			return err
		}

		err = s.UpdateCnameSetupAt(ctx)
		if err != nil {
			return err
		}

		u := goatcounter.User{Site: s.ID, Email: email, Password: []byte(password), EmailVerified: true}
		err = u.Insert(ctx)
		return err
	})
	if err != nil {
		return 2, err
	}

	return 0, nil
}

func findParent(ctx context.Context, p string) (goatcounter.Site, error) {
	var s goatcounter.Site
	id, err := strconv.ParseInt(p, 10, 64)
	if err == nil {
		err = s.ByID(ctx, id)
	} else {
		err = s.ByHost(ctx, p)
	}
	if s.Plan == goatcounter.PlanChild {
		return s, errors.Errorf("can't add child site as parent")
	}
	return s, err
}

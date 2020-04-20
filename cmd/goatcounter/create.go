// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const usageCreate = `
Create a new site and user.

Required flags:

  -domain         Domain to host e.g. "stats.example.com". The site will be
                  available on this domain only, so "stats.example.com" won't be
                  available on "localhost".

  -email          Your email address. Will be required to login.

Other flags:

  -password      Password to log in; will be asked interactively if omitted.

  -name          Name for the site and user; can be changed later in settings.

  -parent        Parent site; either as ID or domain.

  -db            Database connection string. Use "sqlite://<dbfile>" for SQLite,
                 or "postgres://<connect string>" for PostgreSQL
                 Default: sqlite://db/goatcounter.sqlite3

  -createdb      Create the database if it doesn't exist yet; only for SQLite.

  -debug         Modules to debug, comma-separated or 'all' for all modules.
`

func create() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()

	var (
		domain, email, name, parent, password string
		createdb                              bool
	)
	CommandLine.StringVar(&domain, "domain", "", "")
	CommandLine.StringVar(&email, "email", "", "")
	CommandLine.StringVar(&name, "name", "serve", "")
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
			Name:  name,
			Code:  "serve-" + zhttp.Secret()[:10],
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

		u := goatcounter.User{Site: s.ID, Name: name, Email: email, Password: []byte(password), EmailVerified: true}
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
		return s, fmt.Errorf("can't add child site as parent")
	}
	return s, err
}

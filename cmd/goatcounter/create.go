// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"syscall"

	"golang.org/x/term"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zli"
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
               See "goatcounter help debug" for a list of modules.
`

func cmdCreate(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	defer func() { ready <- struct{}{} }()

	var (
		dbConnect = f.String("sqlite://db/goatcounter.sqlite3", "db").Pointer()
		debug     = f.String("", "debug").Pointer()
		domain    = f.String("", "domain").Pointer()
		email     = f.String("", "email").Pointer()
		parent    = f.String("", "parent").Pointer()
		password  = f.String("", "password").Pointer()
		createdb  = f.Bool(false, "createdb").Pointer()
	)
	err := f.Parse()
	if err != nil {
		return err
	}

	return func(dbConnect, debug, domain, email, parent, password string, createdb bool) error {
		zlog.Config.SetDebug(debug)
		cfg.Serve = true

		v := zvalidate.New()
		v.Required("-domain", domain)
		v.Domain("-domain", domain)
		if parent == "" {
			v.Required("-email", email)
			v.Email("-email", email)
		}
		if v.HasErrors() {
			return v
		}

		db, err := connectDB(dbConnect, nil, createdb, true)
		if err != nil {
			return err
		}
		defer db.Close()
		ctx := zdb.WithDB(context.Background(), db)

		err = (&goatcounter.Site{}).ByHost(ctx, domain)
		if err == nil {
			return fmt.Errorf("there is already a site for the domain %q", domain)
		}

		if password == "" {
		getpw:
			fmt.Fprintf(zli.Stdout, "Enter password for new user (will not echo): ")
			pwd1, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return err
			}
			if len(pwd1) < 8 {
				fmt.Fprintln(zli.Stdout, "\nNeed at least 8 characters")
				goto getpw
			}

			fmt.Fprintf(zli.Stdout, "\nConfirm: ")
			pwd2, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return err
			}
			fmt.Fprintln(zli.Stdout, "")

			if !bytes.Equal(pwd1, pwd2) {
				fmt.Fprintln(zli.Stdout, "Passwords did not match; try again.")
				goto getpw
			}

			password = string(pwd1)
		}

		var ps goatcounter.Site
		if parent != "" {
			ps, err = findParent(zdb.WithDB(context.Background(), db), parent)
			if err != nil {
				return err
			}
		}

		err = zdb.TX(ctx, func(ctx context.Context) error {
			d := domain
			s := goatcounter.Site{
				Code:  "serve-" + zcrypto.Secret64(),
				Cname: &d,
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

			if parent == "" {
				u := goatcounter.User{Site: s.ID, Email: email, Password: []byte(password), EmailVerified: true}
				err = u.Insert(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	}(*dbConnect, *debug, *domain, *email, *parent, *password, *createdb)
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

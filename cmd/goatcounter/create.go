// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

import (
	"context"
	"os"

	"zgo.at/goatcounter"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zvalidate"
)

const usageCreate = `
Create a new site and user. This is mostly useful for the "serve" command; if
you're using "saas" you can create a new site/user through in the UI.

You can create (and host) multiple sites like this.

Required flags:

  -domain         Domain to host e.g. "stats.example.com".

  -email          Your email address. Will be required to login.

Other flags:

  -db            Database connection string. Use "sqlite://<dbfile>" for SQLite,
                 or "postgres://<connect string>" for PostgreSQL
                 Default: sqlite://db/goatcounter.sqlite3

  -debug         Modules to debug, comma-separated or 'all' for all modules.

  -plan          Plan for new installations; default: businessplus.

  -name          Name for the site and user; can be changed later in settings.
`

// TODO: maybe just make a new "export-certs" command for this? Or generic even
// part of more generic "export"?
//
//  -certdir       ACME-generated certificates are stored in the SQL database
//                 and can be used by GoatCounter from there. You can set this
//                 to also store certificates in a directory, which makes it
//                 easier to use an external https proxy.

func create() (int, error) {
	dbConnect := flagDB()
	debug := flagDebug()

	var domain, email, plan, name string
	CommandLine.StringVar(&domain, "domain", "", "")
	CommandLine.StringVar(&email, "email", "", "")
	CommandLine.StringVar(&plan, "plan", goatcounter.PlanPersonal, "")
	CommandLine.StringVar(&name, "name", "serve", "")
	CommandLine.Parse(os.Args[2:])

	zlog.Config.SetDebug(*debug)

	v := zvalidate.New()
	v.Required("-domain", domain)
	v.Required("-email", email)
	v.Required("-plan", plan)
	v.Include("-plan", plan, goatcounter.Plans)
	v.Domain("-domain", domain)
	v.Email("-email", email)
	if v.HasErrors() {
		return 1, v
	}

	db, err := connectDB(*dbConnect, nil)
	if err != nil {
		return 2, err
	}
	defer db.Close()

	err = zdb.TX(zdb.With(context.Background(), db), func(ctx context.Context, tx zdb.DB) error {
		s := goatcounter.Site{
			Name:  name,
			Code:  "serve-" + zhttp.Secret()[:10],
			Cname: &domain,
			Plan:  plan,
		}
		err := s.Insert(ctx)
		if err != nil {
			return err
		}

		u := goatcounter.User{Site: s.ID, Name: name, Email: email}
		err = u.Insert(ctx)
		return err
	})
	if err != nil {
		return 2, err
	}

	// TODO: Create certificate; fix ACME first though.
	// acme.Domains <- domain
	// acme.Wait()
	return 0, nil
}

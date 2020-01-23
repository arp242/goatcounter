// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package main

const usageServe = `
Serve existing domains. Set up a site with "create" first.

Flags:

  -db            Database connection string. Use "sqlite://<dbfile>" for SQLite,
                 or "postgres://<connect string>" for PostgreSQL
                 Default: sqlite://db/goatcounter.sqlite3

  -automigrate   Automatically run all pending migrations on startup.

  -listen        Address to listen on.

                 Use setcap(1) to allow listening on :443 and :80 on Linux:
                     setcap 'cap_net_bind_service=+ep' goatcounter

                 Default: localhost

  -smtp          SMTP server for sending login emails and errors.
                 Default: not set, is blank, meaning nothing is sent.

  -emailerrors   Email errors to this address; requires -smtp.
                 Default: not set.

  -debug         Modules to debug, comma-separated or 'all' for all modules.

  -certdir       Directory to store ACME-generated certificate.
                 Default: current directory.
`

func serve() error {
	return nil
}

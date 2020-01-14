[![Build Status](https://travis-ci.org/zgoat/goatcounter.svg?branch=master)](https://travis-ci.org/zgoat/goatcounter)
[![codecov](https://codecov.io/gh/zgoat/goatcounter/branch/master/graph/badge.svg)](https://codecov.io/gh/zgoat/goatcounter)

GoatCounter is a web analytics platform, roughly similar to Google Analytics or
Matomo. It aims to give meaningful privacy-friendly web analytics for business
purposes, while still staying usable for non-technical users to use on personal
websites. The choices that currently exist are between freely hosted but with
problematic privacy (e.g. Google Analytics), hosting your own complex software
or paying $19/month (e.g. Matomo), or extremely simplistic "vanity statistics".

There are two ways to run this: as **hosted service** on [goatcounter.com][www],
*free* for non-commercial use, or run it on your own server.

See [docs/rationale.markdown](docs/rationale.markdown) for some more details on
the *"why?"* of this project.

There's a live demo at [https://stats.arp242.net](https://stats.arp242.net).

Please consider [donating][patreon] if you're self-hosting GoatCounter so I can
pay my rent :-)

[patreon]: https://www.patreon.com/arp242
[www]: https://www.goatcounter.com

Features
--------

- **Privacy-aware**; doesn't track users with unique identifiers and doesn't
  need a GDPR consent notice. Also see the [privacy policy][privacy].

- **Lightweight** and **fast**; adds just 3.5K (1.9K compressed) of extra data
  to your site.

- **Easy**; if you've been confused by the myriad of options and flexibility of
  Google Analytics and Matomo that you don't need then GoatCounter will be a
  breath of fresh air. 

- **Accessibility** is a high-priority feature, and the interface works well
  with screen readers, no JavaScript, and even text browsers (although not all
  features work equally well without JS).

- 100% committed to **open source**; you can see exactly what the code does and
  make improvements.

- **Own your data**; you can always export all data and **cancel at any time**.

[privacy]: https://www.goatcounter.com/privacy

### Technical

- Fast: can handle about 800 hits/second on a $5/month Linode VPS using the
  default settings.

- Self-contained binary: everything – including static assets – is in a single
  ~7M statically compiled binary. The only other thing you need is a SQLite
  database file or PostgreSQL connection (no way around that).

Running your own
----------------

There are binaries on the [releases][release] page, or compile from source with
`go get zgo.at/goatcounter`, which will put the binary at
`~/go/bin/goatcounter`.

Go 1.12 and newer are supported (it follows the [Go release policy][rp]). You
will need a C compiler (for SQLite) or PostgreSQL.

[release]: https://github.com/zgoat/goatcounter/releases
[rp]: https://golang.org/doc/devel/release.html#policy

### Production

1. For a production environment run something like:

       goatcounter \
           -prod \
           -smtp         'smtp://localhost:25' \
           -plan         'pro' \
           -domain       'example.com' \
           -domainstatic 'static.example.com' \
           -emailerrors  'me@example.com' \
           "$@"

   The default is to use a SQLite database at `./db/goatcounter.sqlite3` (will
   be created if it doesn't exist). See the `-dbconnect` flag to customize this.

   The `-prod` flag affects various minor things; without it it'll try to load
   templates from the filesystem (instead of using the built-in ones), for
   example.

   `-smtp` is required to send login emails. You can use something like Mailtrap
   if you just want it for yourself, but you can also use your Gmail or whatnot.

2. Use a proxy for https (e.g. [hitch][hitch] or [caddy][caddy]); you'll need to
   forward `example.com` and `*.example.com`

You can see the [goathost repo][goathost] for the server configuration of
goatcounter.com, although that is just one way of running it.

[hitch]: https://github.com/varnish/hitch
[caddy]: https://caddyserver.com/
[goathost]: https://github.com/zgoat/goathost

### Updating

You may need to run run database migrations when updating. Use  `goatcounter
-migrate auto` to always run all pending migrations on startup. This is the
easiest way, although arguably not the "best" way.

Use `goatcounter -migrate <file>` or `goatcounter -migrate all` to manually run
migrations; generally you want to upload the new version, run migrations while
the old one is still running, and then restart so the new version takes effect.

### PostgreSQL

Both SQLite and PostgreSQL are supported. SQLite should work well for the vast
majority of people and is the recommended database engine. PostgreSQL will not
be faster in most cases, and the chief reason for adding support in the first
place is to support load balancing web requests over multiple servers. To use
it:

1. Create the database, unlike SQLite it's not done automatically:

       $ createdb goatcounter
       $ psql goatcounter -c '\i db/schema.pgsql'

2. Run with `-pgsql` and `-dbconnect`, for example:

       $ goatcounter -pgsql -dbconnect 'user=goatcounter dbname=goatcounter sslmode=disable'

   See the [pq docs][pq] for more details on the connection string.

3. You can compile goatcounter without cgo if you don't use SQLite:

       $ CGO_ENABLED=0 go build

   Functionally it doesn't matter too much, but you won't need a C compiler,
   builds will be faster, and makes creating static binaries easier.

[pq]: https://godoc.org/github.com/lib/pq

### Development

See [.github/CONTRIBUTING.markdown](/.github/CONTRIBUTING.markdown) for details
on how to run a development server, write patches, etc.

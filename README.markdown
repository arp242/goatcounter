[![Build Status](https://travis-ci.org/zgoat/goatcounter.svg?branch=master)](https://travis-ci.org/zgoat/goatcounter)
[![codecov](https://codecov.io/gh/zgoat/goatcounter/branch/master/graph/badge.svg)](https://codecov.io/gh/zgoat/goatcounter)

GoatCounter aims to give meaningful privacy-friendly web analytics for business
purposes, while still staying usable for non-technical users to use on personal
websites. The choices that currently exist are between freely hosted but with
problematic privacy (e.g. Google Analytics), hosting your own complex software
or paying $19/month (e.g. Matomo), or extremely simplistic "vanity statistics".

There are two ways to run this: as **hosted service**, *free* for non-commercial
use, or run it on your own server. Check out [https://www.goatcounter.com][www]
for the hosted service and user documentation.

See [docs/rationale.markdown](docs/rationale.markdown) for some more details on
the *"why?"* of this project.

There's a live demo at [https://stats.arp242.net](https://stats.arp242.net).

Please consider [donating][patreon] if you're self-hosting GoatCounter so I can
pay my rent :-) Also see the [announcement post][launch].

Features
--------

- **Privacy-aware**; doesn't track users; doesn't need a GDPR consent notice.

- **Lightweight** and **fast**; adds just 1.5KB (0.7KB compressed) of extra data
  to your site.

- **Easy**; if you've been confused by the myriad of options and flexibility of
  Google Analytics and Matomo that you don't need then GoatCounter will be a
  breath of fresh air. 

- **Accessibility** is a high-priority feature, and the interface works well
  with screen readers, no JavaScript, and even text browsers.

- 100% committed to **open source**; you can see exactly what the code does and
  make improvements.

- **Own your data**; you can always export all data and **cancel at any time**.

### Technical

- Fast: can handle about 800 hits/second on a $5/month Linode VPS using the
  default settings.

- Self-contained binary: everything (including static assets) is in a single 5M
  statically compiled binary. The only other thing you need is a SQLite database
  file or PostgreSQL connection.

Running your own
----------------

Go 1.12 and newer are supported (it follows the [Go release policy][rp]). You
will need a C compiler (for SQLite) or PostgreSQL.

### Development

1. Install it with:

       $ git clone git@github.com:zgoat/goatcounter.git
       $ cd goatcounter
       $ go build ./cmd/goatcounter

   This will put a self-contained binary at `goatcounter`. You can optionally
   reduce the binary size a bit (from ~19M to ~7M) with `strip` and/or `upx`.

2. Run `./goatcounter`. This will run a development environment on
   http://goatcounter.localhost:8081

   The default is to use a SQLite database at `./db/goatcounter.sqlite3` (will
   be created if it doesn't exist). See the `-dbconnect` flag to customize this.

3. You can sign up your new site at http://www.goatcounter.localhost:8081, which
   can then be accessed at http://[code].goatcounter.localhost:8081

   Note: some systems require `/etc/hosts` entries `*.goatcounter.localhost`,
   whereas others work fine without. If you can't connect try adding this:

       127.0.0.1 goatcounter.localhost www.goatcounter.localhost static.goatcounter.localhost code.goatcounter.localhost

### Production

1. For a production environment run something like:

       goatcounter \
           -prod \
           -plan         'pro' \
           -domain       'goatcounter.com' \
           -domainstatic 'static.goatcounter.com' \
           -smtp         'smtp://localhost:25' \
           -emailerrors  'me@example.com' \
           "$@"

2. Use a proxy for https (e.g. [hitch][hitch] or [caddy][caddy]); you'll need to
   forward `example.com` and `*.example.com`

You can see the [goathost repo][goathost] for the server configuration of
goatcounter.com, although that is just one way of running it.

### Updating

1. `git pull` and build a new version as per above.

2. Database migrations are *not* run automatically, but the app will warn on
   startup if there are migrations that need to be run.

3. Run migrations from `db/migrate/<engine>` with the `sqlite` or `psql`
   commandline tool.

### PostgreSQL

Both SQLite and PostgreSQL are supported. SQLite should work well for the vast
majority of people and is the recommended database engine. PostgreSQL will not
be faster in most cases, and the chief reason for adding support in the first
place is to support load balancing web requests over multiple servers.

To use it:

1. Create the database, unlike SQLite it's not done automatically:

       $ createdb goatcounter
       $ psql goatcounter -c '\i db/schema.pgsql'

2. Run with `-pgsql` and `-dbconnect`, for example:

       $ goatcounter -pgsql -dbconnect 'user=goatcounter dbname=goatcounter sslmode=disable'

   See the [pq docs][pq] for more details on the connection string.

3. You can compile goatcounter without cgo if you don't use SQLite:

       $ CGO_ENABLED=0 go build

   Functionally it doesn't matter too much, but it will allow building static
   binaries, speeds up the builds a bit, and makes builds a bit easier as you
   won't need a C compiler.

[www]: https://www.goatcounter.com
[privacy]: https://goatcounter.com/privacy
[pq]: https://godoc.org/github.com/lib/pq
[goathost]: https://github.com/zgoat/goathost
[patreon]: https://www.patreon.com/arp242
[launch]: https://arp242.net/goatcounter.html
[rp]: https://golang.org/doc/devel/release.html#policy
[hitch]: https://github.com/varnish/hitch
[caddy]: https://caddyserver.com/

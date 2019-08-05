GoatCounter is a web counter.

There are two ways to run this: as **hosted service starting at $3/month**, or
run it on your own server.
Check out [https://GoatCounter.com](https://GoatCounter.com) for the hosted
service and user documentation.

Features
--------

- **Privacy-aware**; doesn’t track users; doesn't need a GDPR notice (probably,
  see [privacy](https://goatcounter.com/privacy)).

- **Lightweight** and **fast**; adds just 0.8KB of extra data to your site.

- **Easy**; if you've been confused by the myriad of options and flexibility of
  Google Analytics and Matomo than you don't need then GoatCounter will be a
  breath of fresh air. 

- **Accessibility** is a high-priority feature, and the interface works well
  with screen readers, no JavaScript, and even text browsers.

- 100% committed to **open source**; you can see exactly what the code does and
  make improvements.

- **Own your data**; you can always export all data and **cancel at any time**.


### Technical

- Fast: can handle about 800 hits/second on a $5/month Linode VPS – which is
  also running hitch and varnish – using the default settings.

- Self-contained binary: everything (including static assets) is in a single 5M
  statically compiled binary.

Running your own
----------------

You will need Go 1.10 or newer and a C compiler (for SQLite) or PostgreSQL.

### Development

1. Install it with `go get zgo.at/goatcounter/cmd/goatcounter`. This will put a
   self-contained binary at `~/go/goatcounter`.

2. Run `~/go/goatcounter`. This will run a development environment on
   http://localhost:8081

  The default is to use a SQLite database at `./db/goatcounter.sqlite3` (will be
  created if it doesn't exist). See the `-dbconnect` flag to customize this.

3. You can sign up your new site at http://localhost:8081, which can then be
   accessed at http://test.localhost:8081

### Production

1. For a production environment run something like:

       goatcounter -prod \
           -sentry "https://...:...@sentry.io/..." \
           -domain "goatcounter.com" \
           -domainstatic "static.goatcounter.com" \
           -smtp "smtp://localhost:25" \
           "$@"

2. Use a proxy for https (e.g. Caddy); you'll need to forward `example.com` and
   `*.example.com`

You can see the [goathost repo](https://github.com/zgoat/goathost) for the
server configuration of goatcounter.com, although that is just one way of
running it.

### PostgreSQL

Both SQLite and PostgreSQL are supported. SQLite should work well for the vast
majority of people and is the recommended database engine. PostgreSQL will not
be faster in most cases, and the chief reason for adding support in the first
place is to support load balancing web requests over multiple servers.

To use it:

1. Create the database, unlike SQLite it's not done automatically:

       $ createdb goatcounter
       $ psql goatcounter -c '\i db/schema.pgsql'

2. Optionally convert the SQLite database:

       $ ./db/export-sqlite.sh ./db/goatcounter.sqlite3 | psql goatcounter

3. Run with `-pgsql` and `-dbconnect`, for example:

       $ goatconter -pgsql -dbconnect 'user=goatcounter dbname=goatcounter sslmode=disable'

   See the [pq docs](https://godoc.org/github.com/lib/pq) for more details on
   the connection string.

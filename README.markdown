GoatCounter is a web counter.


There are two ways to run this: as **hosted service starting at $3/month**, or
run it on your own server. Check out [https://www.goatcounter.com][www] for the
hosted service and user documentation. *Note: not yet online; come back in a few
days if you managed to find this repo somehow*.

There's a live demo at [https://arp242.goatcounter.com][demo].

The current status is *public beta*, or "MVP" (Minimum Viable Product) to get
feedback from others. That being sad, it should be stable and useful. It just
doesn't have all the features I'd like it to have (yet).

---

Please consider donating if you run your own instance of GoatCounter.

Basically I quit my day job to try and make a living from creating open source
software full-time (or free software, if you prefer). So supporting isn't just a
nice way to say "thanks mate", it's directly supporting future development.

Features
--------

- **Privacy-aware**; doesn't track users; doesn't need a GDPR notice (probably,
  see [privacy][privacy]).

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

1. Install it with:

       $ git clone git@github.com:zgoat/goatcounter.git
       $ cd goatcounter
       $ go build ./cmd/goatcounter

   This will put a self-contained binary at `goatcounter`. You can optionally
   reduce the binary size a bit (from ~18M to ~5M) with `strip` and/or `upx`.

2. Run `~/go/goatcounter`. This will run a development environment on
   http://localhost:8081

   The default is to use a SQLite database at `./db/goatcounter.sqlite3` (will be
   created if it doesn't exist). See the `-dbconnect` flag to customize this.

3. You can sign up your new site at http://localhost:8081, which can then be
   accessed at http://test.localhost:8081

### Production

1. For a production environment run something like:

       goatcounter \
           -prod \
           -sentry "https://...:...@sentry.io/..." \
           -domain "goatcounter.com" \
           -domainstatic "static.goatcounter.com" \
           -smtp "smtp://localhost:25" \
           "$@"

2. Use a proxy for https (e.g. Caddy); you'll need to forward `example.com` and
   `*.example.com`

You can see the [goathost repo][goathost] for the server configuration of
goatcounter.com, although that is just one way of running it.

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

   See the [pq docs][pq] for more details on the connection string.


[www]: https://www.goatcounter.com
[demo]: https://arp242.goatcounter.com
[privacy]: https://goatcounter.com/privacy
[pq]: https://godoc.org/github.com/lib/pq
[goathost]: https://github.com/zgoat/goathost

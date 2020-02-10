[![Build Status](https://travis-ci.org/zgoat/goatcounter.svg?branch=master)](https://travis-ci.org/zgoat/goatcounter)

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

- **Lightweight** and **fast**; adds just ~3K (~1.5K compressed) of extra data to
  your site. Also has JavaScript-free "tracking pixel" option, or you can use it
  from your application's middleware.

- **Easy**; if you've been confused by the myriad of options and flexibility of
  Google Analytics and Matomo that you don't need then GoatCounter will be a
  breath of fresh air.

- **Accessibility** is a high-priority feature, and the interface works well
  with screen readers, no JavaScript, and even text browsers (although not all
  features work equally well without JS).

- 100% committed to **open source**; you can see exactly what the code does and
  make improvements.

- **Own your data**; you can always export all data and **cancel at any time**.

- Integrate on your site with just a **single script tag**:

      <script data-goatcounter="https://yoursite.goatcounter.com/count"
              async src="//gc.zgo.at/count.js"></script>

- The JavaScript integration is a good option for most, but you can also use a
  **no-JavaScript image-based tracker** or integrate in your **backend
  middleware**.

[privacy]: https://www.goatcounter.com/privacy

### Technical

- Fast: can handle about 800 hits/second on a $5/month Linode VPS using the
  default settings.

- Self-contained binary: everything – including static assets – is in a single
  ~7M statically compiled binary. The only other thing you need is a SQLite
  database file or PostgreSQL connection (no way around that).

Running your own
----------------

There are binaries on the [releases][release] page, or compile from source with:

	$ git clone git@github.com:zgoat/goatcounter.git
	$ cd goatcounter
	$ go build ./cmd/goatcounter

You'll now have a `goatcounter` binary in the current directory.

The master branch should be reasonably stable. You can build/run a specific
release by checking out the tag: `git checkout v1.0.0`.

It's not recommended to use `go get` in GOPATH mode since that will ignore the
versions in go.mod.

Go 1.12 and newer are supported (it follows the [Go release policy][rp]). You
will need a C compiler (for SQLite) or PostgreSQL.

[release]: https://github.com/zgoat/goatcounter/releases
[rp]: https://golang.org/doc/devel/release.html#policy

### Production

For a production environment run something like:

    goatcounter serve \
       -smtp         'smtp://localhost:25' \
       -emailerrors  'me@example.com'

The default is to use a SQLite database at `./db/goatcounter.sqlite3` (will be
created if it doesn't exist). See the `-db` flag to customize this.

`-smtp` is required to send login emails. You can use something like Mailtrap if
you just want it for yourself, but you can also use your Gmail or whatnot.

You can create new sites with the `create` command:

	goatcounter create -email me@example.com -domain stats.example.com

### Updating

You may need to run run database migrations when updating. Use  `goatcounter
-automigrate` to always run all pending migrations on startup. This is the
easiest way, although arguably not the "best" way.

Use `goatcounter migrate <file>` or `goatcounter migrate all` to manually run
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

       $ goatcounter \
           -db           'postgresql://user=goatcounter dbname=goatcounter sslmode=disable' \
           -smtp         'smtp://localhost:25' \
           -emailerrors  'me@example.com'

   See the [pq docs][pq] for more details on the connection string.

3. You can compile goatcounter without cgo if you don't use SQLite:

       $ CGO_ENABLED=0 go build

   Functionally it doesn't matter too much, but you won't need a C compiler,
   builds will be faster, and creating static binaries will be easier.

[pq]: https://godoc.org/github.com/lib/pq

### Development

See [.github/CONTRIBUTING.markdown](/.github/CONTRIBUTING.markdown) for details
on how to run a development server, write patches, etc.

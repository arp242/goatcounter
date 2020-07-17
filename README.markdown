[![Build Status](https://travis-ci.com/zgoat/goatcounter.svg?branch=master)](https://travis-ci.com/zgoat/goatcounter)

GoatCounter is an open source web analytics platform available as a hosted
service (free for non-commercial use) or self-hosted app. It aims to offer easy
to use and meaningful privacy-friendly web analytics as an alternative to Google
Analytics or Matomo.

There are two ways to run this: as **hosted service** on [goatcounter.com][www],
*free* for non-commercial use, or run it on your own server (the source code is
completely Open Source/Free Software, and it can be self-hosted without
restrictions).

See [docs/rationale.markdown](docs/rationale.markdown) for some more details on
the *"why?"* of this project.

There's a live demo at [https://stats.arp242.net](https://stats.arp242.net).

Please consider [contributing financially][sponsor] if you're self-hosting
GoatCounter so I can pay my rent :-)

GoatCounter is sponsored by a grant from [NLnet's NGI Zero PET fund][nlnet].

[nlnet]: https://nlnet.nl/project/GoatCounter/
[sponsor]: http://www.goatcounter.com/contribute
[www]: https://www.goatcounter.com

Features
--------

- **Privacy-aware**; doesn't track users with unique identifiers and doesn't
  need a GDPR consent notice. Also see the [privacy policy][privacy].

- **Lightweight** and **fast**; adds just ~5K (~2.5K compressed) of extra data
  to your site. Also has JavaScript-free "tracking pixel" option, or you can use
  it from your application's middleware.

- **Easy**; if you've been confused by the myriad of options and flexibility of
  Google Analytics and Matomo that you don't need then GoatCounter will be a
  breath of fresh air.

- Identify **unique visits** without cookies using a non-identifiable hash
  ([technical details][sessions]).

- Keeps useful statistics such as **browser** information, **location**, and
  **screen size**. Keep track of **referring sites** and **campaigns**.

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
[sessions]: https://github.com/zgoat/goatcounter/blob/master/docs/sessions.markdown

### Technical

- Fast: can handle about 800 hits/second on a $5/month Linode VPS using the
  default settings.

- Self-contained binary: everything – including static assets – is in a single
  ~7M statically compiled binary. The only other thing you need is a SQLite
  database file or PostgreSQL connection (no way around that).

Running your own
----------------

The [release page][releases] has binaries for Linux amd64, arm, and arm64. These
are statically compiled and should work in pretty much any Linux environment.
GoatCounter should run on any platform supported by Go, but there are no
binaries for them (yet); you'll have to build from source for now (it's not
hard, I promise).

Note this README is for the latest master; use the [`release-1.3`][r-1.3] branch
for the 1.3 README.

Generally speaking only the latest release is supported, although critical fixes
(security, data loss, etc.) may get backported to previous releases.

[releases]: https://github.com/zgoat/goatcounter/releases
[r-1.3]: https://github.com/zgoat/goatcounter/tree/release-1.3

### Building from source

Compile from source with:

    $ git clone -b release-1.3 https://github.com/zgoat/goatcounter.git
    $ cd goatcounter
    $ go build ./cmd/goatcounter

Or to build a statically linked binary:

    $ go build \
        -tags osusergo,netgo,sqlite_omit_load_extension \
        -ldflags='-extldflags=-static' \
        ./cmd/goatcounter

You'll now have a `goatcounter` binary in the current directory.

You need Go 1.13 or newer and a C compiler (for SQLite), or compile it with
`CGO_ENABLED=0 go build` and use PostgreSQL.

It's recommended to use the latest release as in the above command. The master
branch should be reasonably stable but no guarantees, and sometimes I don't
write detailed release/upgrade notes until the actual release.

It's not recommended to use `go get` in GOPATH mode since that will ignore the
dependency versions in go.mod.

### Running

You can start a server with:

    $ goatcounter serve

The default is to use a SQLite database at `./db/goatcounter.sqlite3`, which
will be created if it doesn't exist yet. See the `-db` flag and
`goatcounter help db` to customize this.

GoatCounter will listens on port `*:80` and `*:443` by default. You don't need
to run it as root and can grant the appropriate permissions on Linux with:

    $ setcap 'cap_net_bind_service=+ep' goatcounter

Listening on a different port can be a bit tricky due to the ACME/Let's Encrypt
certificate generation; `goatcounter help listen` documents this in depth.

You can create new sites with the `create` command:

    $ goatcounter create -email me@example.com -domain stats.example.com

This will ask for a password for your new account; you can also add a password
on the commandline with `-password`. If you use a custom DB, you must also pass
the `-db` flag here.

### Updating

You may need to run run database migrations when updating. Use  `goatcounter
-automigrate` to always run all pending migrations on startup. This is the
easiest way, although arguably not the "best" way.

Use `goatcounter migrate <file>` or `goatcounter migrate all` to manually run
migrations; generally you want to upload the new version, run migrations while
the old one is still running, and then restart so the new version takes effect.

Use `goatcounter migrate show` to get a list of pending migrations.

### PostgreSQL

Both SQLite and PostgreSQL are supported. SQLite should work well for the vast
majority of people and is the recommended database engine. PostgreSQL will not
be faster in most cases, and the chief reason for adding support in the first
place is to support load balancing web requests over multiple servers. To use
it:

1. Create the database, unlike SQLite it's not done automatically (you may need
   to modify the `-db` flag):

       $ createdb goatcounter
       $ psql goatcounter -c '\i db/schema.pgsql'
       $ goatcounter -db 'postgresql://dbname=goatcounter' migrate all

2. Run with custom `-db` flag:

       $ goatcounter serve \
           -db 'postgresql://user=goatcounter dbname=goatcounter sslmode=disable'

   See the [pq docs][pq] for more details on the connection string.

3. You can compile goatcounter without cgo if you don't use SQLite:

       $ CGO_ENABLED=0 go build

   Functionally it doesn't matter too much, but builds will be a bit easier and
   faster as it won't require a C compiler.

[pq]: https://godoc.org/github.com/lib/pq

### Development/testing

You can start a test/development server with:

    $ goatcounter serve -dev

The `-dev` flag makes some small things a bit more convenient for development;
TLS is disabled by default, it will listen on localhost:8081, the application
will automatically restart on recompiles, and a few other minor changes.

See [.github/CONTRIBUTING.markdown](/.github/CONTRIBUTING.markdown) for more
details on how to run a development server, write patches, etc.

Various aggregate data files are available at https://www.goatcounter.com/data

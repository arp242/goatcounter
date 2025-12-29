GoatCounter is an open source web analytics platform available as a (free)
hosted service or self-hosted app. It aims to offer easy to use and meaningful
privacy-friendly web analytics as an alternative to Google Analytics or Matomo.

There are two ways to run this: as hosted service on [goatcounter.com][www], or
run it on your own server. The source code is completely Open Source/Free
Software, and it can be self-hosted without restrictions.

See [docs/rationale.md](docs/rationale.md) for some more details on the *"why?"*
of this project.

There's a live demo at [https://stats.arp242.net](https://stats.arp242.net).

Please consider [contributing financially][sponsor].

[sponsor]: http://www.goatcounter.com/contribute
[www]: https://www.goatcounter.com


Features
--------
- **Privacy-aware**; doesn’t track users with unique identifiers and doesn't
  need a GDPR notice. Fine-grained **control over which data is collected**.
  Also see the [privacy policy][privacy] and [GDPR consent notices][gdpr].

- **Lightweight** and **fast**; adds just ~3.5K of extra data to your site. Also
  has JavaScript-free "tracking pixel" option, or you can use it from your
  application's middleware or **import from logfiles**.

- Identify **unique visits** without cookies using a non-identifiable hash
  ([technical details][sessions]).

- Keeps useful statistics such as **browser** information, **location**, and
  **screen size**. Keep track of **referring sites** and **campaigns**.

- **Easy**; if you've been confused by the myriad of options and flexibility of
  Google Analytics and Matomo that you don't need then GoatCounter will be a
  breath of fresh air.

- **Accessibility** is a high-priority feature, and the interface works well
  with assistive technology such as screen readers.

- 100% committed to **open source**; you can see exactly what the code does and
  make improvements, or <strong>self-host</strong> it for any purpose.

- **Own your data**; you can always export all data and **cancel at any time**.

- Integrate on your site with just a **single script tag**:

      <script data-goatcounter="https://yoursite.goatcounter.com/count"
              async src="//gc.zgo.at/count.js"></script>

- The JavaScript integration is a good option for most, but you can also use a
  **no-JavaScript image-based tracker**, integrate it in your **backend
  middleware**, or **parse log files**.

[privacy]: https://www.goatcounter.com/privacy
[gdpr]: https://www.goatcounter.com/gdpr
[sessions]: http://www.goatcounter.com/help/sessions


Getting data in to GoatCounter
------------------------------
There are three ways:

1. Add the JavaScript code on your site; this is the easiest and most common
   method. Detailed documentation for this is available at
   https://www.goatcounter.com/code

2. Use the HTTP/REST API, for example from your backend server middleware.
   Detailed documentation for this is available at
   https://www.goatcounter.com/api#backend-integration

3. Parse logfiles of nginx, Apache, Caddy, CloudFront, or any other HTTP server
   or proxy. See `goatcounter help import` for detailed documentation on this
   (this works both for the self-hosted version and goatcounter.com).

Self-hosting GoatCounter
------------------------
The [release page][releases] has binaries for several platforms. These are
statically compiled and contain everything you need. These should work in pretty
much any environment. The only dependency is somewhere to store a SQLite
database file or a PostgreSQL connection. Alternatively you can use Docker, as
documented in the section below.

[releases]: https://github.com/arp242/goatcounter/releases

### Running
You can start a server with:

    % goatcounter serve

This will start a server on `*:8080`. The default is to use an SQLite database
at `./goatcounter-data/db.sqlite3`, which will be created if it doesn't exist
yet.

Both SQLite and PostgreSQL are supported. SQLite should work well for most
smaller sites, but PostgreSQL gives better performance especially for larger
sites. There are [some benchmarks over here][bench] to give some indication of
what performance to expect from SQLite and PostgreSQL.

To create the first site, use the wizard on http://localhost:8080 or the CLI
with:

    % goatcounter db create site -vhost=stats.example.com -user.email=me@example.com

This will ask for a password; you can also add a password on the commandline
with `-password`. You must also pass the `-db` flag here if you use something
other than the default.

GoatCounter includes TLS and automatic ACME certificate generation; to run in
production you probably want something like:

    % goatcounter serve -listen=:443 -tls=tls,rdr,acme

See `goatcounter help serve` for details.

[bench]: https://github.com/arp242/goatcounter/blob/main/docs/benchmark.md

### PostgreSQL
To use PostgreSQL, run GoatCounter with a custom `-db` flag. For example:

    % goatcounter serve -db 'postgresql+dbname=goatcounter'
    % goatcounter serve -db 'postgresql+host=/run/postgresql dbname=goatcounter sslmode=disable'

This follows the format in the `psql` CLI; you can also use the `PG*`
[environment variables](https://www.postgresql.org/docs/current/libpq-envars.html):

    % PGDATABASE=goatcounter PGHOST=/run/postgresql goatcounter serve -db 'postgresql'

The database will be created automatically if possible; if you want to create it
for a specific user you can use:

    % createuser --interactive --pwprompt goatcounter
    % createdb --owner goatcounter goatcounter

You can manually import the schema with:

    % goatcounter db schema-pgsql | psql --user=goatcounter --dbname=goatcounter

See `goatcounter help db` and the [pgx docs] for more details.

[pgx docs]: https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#ParseConfig

### Running with Docker
GoatCounter is available on DockerHub at [arp242/goatcounter].

Example to run a new container:

    % docker run \
        -p 8080:8080 \
        -v goatcounter-data:/home/goatcounter/goatcounter-data \
        arp242/goatcounter

This uses a named volume, which is recommended as this stores the SQLite
database and ACME certificates (when using ACME) and anonymous volumes can be
easy to accidentally delete.

To create the first site, use the wizard on http://localhost:8080 or the CLI
with:

    % docker exec -it [..] goatcounter db create site -vhost=stats.example.com -user.email=me@example.com

To set options you can use `GOATCOUNTER_..` environment variables. For example
to enable TLS and automatic certificate generation:

    % docker run \
        -p 80:80 \
        -p 443:443 \
        -v goatcounter-data:/home/goatcounter/goatcounter-data \
        -e GOATCOUNTER_LISTEN=:443 \
        -e GOATCOUNTER_TLS=tls,rdr,acme \
        arp242/goatcounter

Set `GOATCOUNTER_DB` to use PostgreSQL. For example:

    % docker run \
        -p 8080:8080 \
        -v goatcounter-data:/home/goatcounter/goatcounter-data \
        -e GOATCOUNTER_DB='postgresql+postgresql://goatcounter:goatcounter@postgres:5432/goatcounter?sslmode=disable' \
        arp242/goatcounter

See `goatcounter help serve` (or: `docker run --rm arp242/goatcounter help serve`)
for all options.

All of the above should also work with Podman.

You can also run GoatCounter from compose.yaml with `docker compose`. For a
basic SQLite setup:

    % docker compose up -d goatcounter-sqlite

Or PostgreSQL (also starts PostgreSQL from compose.yaml):

    % docker compose up -d goatcounter-postgres

[arp242/goatcounter]: https://hub.docker.com/r/arp242/goatcounter

### Updating
You may need to run the database migrations when updating. Use  `goatcounter
serve -automigrate` to always run all pending migrations on startup.

Use `goatcounter db migrate <file>` or `goatcounter db migrate all` to manually run
migrations.

Use `goatcounter db migrate pending` to get a list of pending migrations, or
`goatcounter db migrate list` to show all migrations.

### Building from source
You need Go 1.21 or newer and a C compiler. If you compile it with
`CGO_ENABLED=0` you don't need a C compiler but can only use PostgreSQL.

You can build from source with:

    % git clone --branch=release-2.7 https://github.com/arp242/goatcounter
    % cd goatcounter
    % go build ./cmd/goatcounter

Which will produce a `goatcounter` binary in the current directory.

To use the latest development version switch to the `main` branch.

To build a fully statically linked binary:

    % go build -trimpath -ldflags='-s -w -extldflags=-static' \
        -tags='osusergo,netgo,sqlite_omit_load_extension' \
        ./cmd/goatcounter

It's recommended to use the latest release as in the above command. The main
branch should be reasonably stable but no guarantees, and sometimes I don't
write detailed release/upgrade notes until the actual release so you may run in
to surprises.

You can compile goatcounter without cgo if you're planning to use PostgreSQL and
don't use SQLite:

    % CGO_ENABLED=0 go build ./cmd/goatcounter

This will create a statically linked binary by default; no extra flags needed.
Functionally it doesn't matter too much, but builds will be a bit easier and
faster as you won't need a C compiler.

### Development/testing
You can start a test/development server with:

    % goatcounter serve -dev

The `-dev` flag makes some small things a bit more convenient for development:
the application will automatically restart on recompiles, templates and static
files will be read directly from the filesystem, and a few other minor changes.

See [.github/CONTRIBUTING.md](/.github/CONTRIBUTING.md) for more details on how
to run a development server, write patches, etc.

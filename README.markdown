GoatCounter is a web counter.

There are two ways to run this: as **hosted service for $3/month**, or run it
on your own server. Check out [https://GoatCounter.com](https://GoatCounter.com)
for the hosted service and user documentation.


Running your own
----------------

You will need Go 1.10 or newer and a C compiler (for SQLite) or PostgreSQL.

### Development

1. Install it with `go get zgo.at/goatcounter/cmd/goatcounter`. This will put a
   self-contained binary at `~/go/goatcounter`.

2. Run `~/go/goatcounter`. This will run a developmnet environment on
   http://localhost:8081

  The default is to use a SQLite database at `./db/goatcounter.sqlite3` (will be
  created if it doesn't exist). See the `-dbconnect` flag to customize this.

3. You can sign up your new site at http://localhost:8081, which can then be
   accessed at http://test.localhost:8081

### Production

1. For a production environment run something like:

       goatcounter \
           -prod \
           -domain example.com \
           -domainstatic static.example.com

2. Use a proxy for https (e.g. Caddy); you'll need to forward `example.com` and
   `*.example.com`

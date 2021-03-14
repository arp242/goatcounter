You can get in touch on GitHub issues or the [Telegram group][t] if you have any
questions or problems.

Don't be afraid to just ask if you're struggling with something. Chances are
that I can quickly give you an answer or point you in the right direction by
spending just a few minutes.

[t]: https://t.me/goatcounter

Starting it
-----------

1. Various files like static assets and templates are loaded from the filesystem
   if they exist; you should run GoatCounter from the goatcounter source
   directory on development.

2. Running `goatcounter serve -dev` will run a development environment on
   http://goatcounter.localhost:8081

3. You can sign up your new site at http://www.goatcounter.localhost:8081, which
   can then be accessed at http://[code].goatcounter.localhost:8081

   Note: some systems require `/etc/hosts` entries `*.goatcounter.localhost`,
   whereas others work fine without. If you can't connect try adding this:

       127.0.0.1 goatcounter.localhost www.goatcounter.localhost static.goatcounter.localhost code.goatcounter.localhost


General notes
-------------

- It's probably best to create an issue first for non-trivial patches. This
  might prevent you from wasting time on a wrong approach, or from working on
  something that will never be merged.

  - I'm pretty wary of introducing dependencies, especially if they come with
    large dependency trees. So it's probably best to communicate in the issue if
    you're planning to do that.

- Use `-debug <mod>` to enable debug logs for specific modules, or `-debug all`
  to enable it for all modules.

- Automatic reload is managed with github.com/teamwork/reload. Basically it will
  restart on recompile, and reload templates once they change (no restart
  required).

- Tests can be run with `go test ./...`; nothing special needed. You can run
  tests against PostgreSQL (instead of SQLite) with `go test -tags=testpg
  ./...`. You can use the standard `PG*` environment variables to control the
  connection (e.g. `PGHOST`, `PGPORT`).

- Use `go run ./cmd/check ./...` to run some various linters and the like, such
  as `go vet` and some others.

- Keep lines under 80 characters if possible, but don't bend over backwards to
  do so: it's usually okay for a function definition or call to be 85 characters
  or whatnot. Comments should pretty much always be wrapped to 80 characters
  though.


Code design
-----------

Some notes about the code; most of it should – hopefully – be fairly
straightforward

- cmd/goatcounter is the `main` package which starts everything.

- The "models" are contained in /site.go, /hit.go, etc. [site.go](/site.go) is
  probably the best place to look at to get an overview of the patterns used.
  Most of this is fairly straight-forward and uncomplicated.

- HTTP handlers go in /handlers.

- `/count` – which records pageviews – is dealt different than most other
  requests: instead of persisting to the DB immediately it's added to memstore
  first. The cron package will persist that to DB every 10 seconds, which also
  regenerates various cached stats.

- Hits ("pageviews") are stored in the `hits` table with minimal processing; for
  the most part, this table isn't queried directly for reasons of performance.
  When inserting new rows in the table the various `*_stats` and `*_count`
  tables are updated which contain a more efficient aggregation of the data
  (`hit_stats`, browser_stats`, etc.) This is done through the `cron` package.

- Templates live in `/tpl`, and are standard Go templates. The Go template
  library is a bit idiosyncratic, but once you "get" it they're quite pleasant
  to work with (I can't find a good/gentle "getting started with Go templates"
  introduction, let me know if you know of one; but just ask if you're
  struggling with this).

- The frontend is in /public. It's all simple basic CSS with simple jQuery-based
  JavaScript.


Special cookies
---------------

These only work in `-dev` mode:

- Set the `debug-delay` cookie to a numerical value to delay the response of
  every request by *n* seconds. This is mostly intended to debug frontend timing
  issues.

- Set the `debug-explain` cookie to automatically print all queries and their
  EXPLAIN. If this is an empty string everything will be printed, and if it's
  non-empty only queries containing the given string will be printed.

Pro-tip: setting cookies in the debugger tools is a bit of a pain; I tend to
just set these cookies once, and set the path to `/asdasd` to "disable" then,
and back to `/` if I want to enable it again :-)

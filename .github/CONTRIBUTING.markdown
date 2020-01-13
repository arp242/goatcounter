You can get in touch on GitHub issues or the
[Telegram group](https://t.me/goatcounter) if you have any questions or
problems.

Don't be afraid to just ask if you're struggling with something. Chances are
that I can quickly give you an answer or point you in the right direction by
spending just a few minutes.

Starting it
-----------

1. In development mode (i.e. without `-prod`) various files like static assets
   and templates are loaded from the filesystem. For this reason goatcounter
   expects to be run from the goatcounter source directory.

2. Running `goatcounter` without flags will run a development environment on
   http://goatcounter.localhost:8081

3. You can sign up your new site at http://www.goatcounter.localhost:8081, which
   can then be accessed at http://[code].goatcounter.localhost:8081

   Note: some systems require `/etc/hosts` entries `*.goatcounter.localhost`,
   whereas others work fine without. If you can't connect try adding this:

       127.0.0.1 goatcounter.localhost www.goatcounter.localhost static.goatcounter.localhost code.goatcounter.localhost

4. Don't forget to run `go generate ./...` before building a release binary;
   this will generate the `pack/pack.go` file which contains all static assets
   (JS/CSS, templates, DB migrations).


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

- Tests can be run with `go test ./...`; nothing special needed.

- Run `go generate ./...` before committing; this will generate the
  `pack/pack.go` file, which contains all the static resources for production
  use (so it can be deployed as a self-contained binary).

- I don't run any linters at the moment other than `go vet`, as several years of
  experience with them showed that they're useful about half the time, and just
  noise the other half. But do try to follow standard go linter guidelines when
  reasonable and, of course, gofmt your code.

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

- HTTP handlers go in /handlers; most of the interesting stuff is in
  [handlers/backend.go](/handlers/backend.go).

- `/count` – which records pageviews – is dealt different than most other
  requests: instead of persisting to the DB immediately it's added to memstore
  first. The cron package will persist that to DB every 10 seconds, which also
  regenerates various cached stats.

- Templates live in /tpl, and are standard Go templates. The Go template library
  is a bit idiosyncratic, but once you "get" it they're quite pleasant to work
  with (I can't find a good/gentle "getting started with Go templates"
  introduction, let me know if you know of one; but just ask if you're
  struggling with this).

- The frontend is in /public. It's all simple basic CSS with simple jQuery-based
  JavaScript.

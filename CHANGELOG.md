ChangeLog for GoatCounter
=========================
This list is not comprehensive, and only lists new features and major changes,
but not every minor bugfix. The goatcounter.com service generally runs the
latest main.

unreleased
----------

### Features

- Include support for fetching GeoDB updates from MaxMind.

  The `-geodb` flag now accepts `maxmind:account_id:license` to automatically
  download updates. See `goatcounter help serve` for the full documentation.

- Add buttons to navigate by year on the dashboard.

- Automatically detect cookie `secure` and `sameSite` attributes. Previously
  this relied on the correct usage of the `-tls` flag, which people often got
  wrong. Now it's detected from the client connection.

  This depends on the proxy to set `Scheme: https` or `X-Forwarded-Proto: https`
  header, which most should already do by default.

- WebSocket support is now detected automatically, without the need to set the
  `-websocket` flag (which is now a no-op).

- Store bot pageviews in new "bots" table for 30 days. This table is never used,
  but it can be useful for debugging purposes.

- Add more detailed totals to /api/v0/stats/totals. Previously it would only
  return the grand totals; now it also returns the totals broken down by hour
  and day.

- Allow finding paths by name in the API.

  Everything that accepts include_paths=.. and exclude_paths=.. now also accepts
  path_by_name=true to finds paths by the path rather than ID.

- Read `GOATCOUNTER_TMPDIR` environment variable as an alternative way to set
  `TMPDIR`. Mainly intended for cases where `TMPDIR` can't be used (e.g. when
  the capability bit is set on Linux).

- Use PostgreSQL 17 in compose.yaml; also update the PostgreSQL settings to be
  less conservative.

### Fixes

- Adjust screen size categories for more modern devices.

- API Tokens are now shared between all sites.

- Include all sites in email reports, instead of just the first site that was
  created.

- Fix merging of multiple paths when more than once path has entries for the
  same hour.

- Store Campaign in hits table.

- Make sure the visitor counter works for events.

- Set CORS headers for API.

- Pass `hideui=1` when redirecting with access-token.

- Fix pagination of refs.

- Fix page count for text table after pagination.

- Don't store rows with total=0 in {hit,ref}_counts.

- Make sure CLI flags with a `.` such as `-user.email` can be set from
  environment.

- Don't add `translate-to=..` to query parameters. Previously it would set this
  from Google Translate parameters so you could see which languages people were
  using with that, but I don't think anyone ever used that and it just added
  paths for no real reason.

- Remove sizes table; was only used for `hits.size_id`, which is now replaced
  with `hits.width`. This indirection didn't really add much.

- Correctly display error on widgets; previously it would just display
  `errors.errorString Value`.

- Disable daily view if less than 7 days are selected.

2025-06-08 v2.6.0
-----------------
This release changes a number of default values. In most cases this shouldn't
break anything, but be sure to read the section.

This release requires Go 1.21.

### Changes in defaults

- The default values for the `-listen` and `-tls` flags have changed from
  `-listen=:443 -tls=tls,rdr,acme` to `-listen=:8080 -tls=none`.

  The previous defaults were "production ready", but in practice many people
  don't use the built-in TLS and ACME certificate generation but a proxy like
  nginx or Caddy. In addition, it's also easier to get started with the new
  defaults.

- The default SQLite database location changed from `./db/goatcounter.sqlite3`
  to `./goatcounter-data/db.sqlite3`. The old file will still be used as a
  default if it exists, so this shouldn't break any existing setups.

- The default ACME secrets location changed from `./acme-secrets` to
  `./goatcounter-data/acme-secrets`. The old directory will still be used as a
  default if it exists, so shouldn't break any existing setups.

- No longer check for `window.goatcounter.vars` and `window.counter` in
  `count.js`. These were changed a week or so after the initial release over
  five years ago. AFAIK no one is using them.

  If you do, then use `window.goatcounter` (or `data-goatcounter-settings`)
  instead of `window.goatcounter.vars` and `data-goatcounter="url"` on the
  script tag instead of `window.counter`.

- No longer store individual pageviews in the `hits` table by default.

  The pageviews in the `hits` table are never used for displaying the dashboard,
  only for the CSV export. Not storing it has two advantages:

  - More privacy-friendly; we only store aggregate data, not exact data.
  - Uses less disk space, potentially a lot less for larger sites.

  The downsides are:

  - It may make debugging a bit harder in some cases.
  - Exporting pageviews won't be possible, because this data no longer exist.
    You can still use the API to get aggregate data.

  Storing individual pageviews in the `hits` table can be enabled from *Settings
  → Data Collection → Individual pageviews*. Existing sites with recent exports
  should have it enabled automatically.

  Existing data isn't removed, so you will have to manually run `truncate hits`
  or `delete from hits` if you want to free up some disk space.

### Features

- Include Dockerfile and publish images on DockerHub. See README for details.

- Take default values of CLI flags from environment variables as
  `GOATCOUNTER_«FLAG»`, where «FLAG» is identical to the CLI flag name. The CLI
  always takes precedence.

- Automatically load GeoIP database from `./goatcounter-data/` directory if it
  exists; it automatically loads the first .mmdb file.

- Improve log parsing:

  - Add more formats for `-datetime`: `unix_sec`, `unix_milli`, `unix_nano`,
    `unix_sec_float`, and `unixmilli_float`.

  - Add `$url` format specifier.

  - Add `bunny` and `caddy` log formats.

- Improve dark theme, and enable by default if set in browser/system
  preferences.

- Add translations for Chinese, Korean.

- Add HTTP2 Cleartext (h2c) handler to improve compatibility with some proxies.

- Add `-base-path` flag to allow running GoatCounter under a different path such
  as `example.com/stats`.

- Allow importing Google Analytics reports. Google Analytics doesn't really
  offer a meaningful export, but does allow exporting "reports" with the totals
  per path. We can't show anything useful on the dashboard, but we can use it to
  show correct totals on the visitor counter.

- Sites are no longer soft-deleted for 7 days. The deletion is still as a
  background job as it may take a while.

### Fixes

- Use img-based fallback if sendBeacon fails in count.js. This helps with some
  sites that forbid using connect-src from the CSP (e.g. neocities).

- Disable keyboard input on datepicker. Previously the arrow keys would move the
  date, but this was more annoying than anything else and prevented manually
  twiddling the text.

- Set CORS on the visitor counter (`/counter/[..].json)` in case of errors. It
  would only set `Access-Control-Allow-Origin` if the operation succeeded, but
  not on errors so you'd never see the error.

- Strip trailing slash from visitor counter. Trailing slashes are always
  stripped for paths in the dashboard, so do it in the visitor counter as well.

- Better error if SQLite DB directory isn't writable when creating a new
  database. SQLite doesn't try to create the file until the first SQL command,
  which happens to be the version check. This would fail with a confusing
  `requires SQLite 3.35.0 or newer; have ""` error.

- Better styling when printing the dashboard.

- Correctly populate the `languages` table when creating a new database.
  Previously collecting language statistics didn't work correct due to this.

2023-12-10 v2.5.0
-----------------
This release requires Go 1.21.

Features:

- Quite a few tables are rewritten to a more efficient format. For small to
  medium instances this will take a few minutes at the most, but if you have
  very large instances this may take a few hours. It also requires enough free
  disk space to rewrite the `hits` table.

  If you want to run steps manually then you can view the migration with:

      % goatcounter db migrate -show 2023-05-16-1-hits

  Or if you use PostgreSQL:

      % goatcounter db migrate -show -db postgresql+dbname=goatcounter 2023-05-16-1-hits

- The `User-Agent` header is no longer stored; only the browser and system
  parsed out of there. It's pretty reliable, and especially mobile browser
  User-Agents are ridiculously unique. It was always stored only "in case the
  detection got it horribly wrong", but this has never been needed.

- Add `proxy` option in `serve -tls` flag,  to give a hint that a secure
  connection will be used, so we know what value to use for the cookie
  secure/samesite flags.

- Add *experimental* "dark mode"; this needs to be enabled explicitly in the
  user settings. I need help to make this decent:
  https://github.com/arp242/goatcounter/issues/586#issuecomment-1287995673

- Show difference of pageviews compared to previous period on the dashboard.

- Make setup of a new installation a bit easier: instead of telling people to
  use the CLI, display a form when the database is 100% empty.

- Add support for JSON logs with the `-json` flag. The `version` command also
  accepts a `-json` flag for outputting the version information as JSON. Also
  change the default text logging to look a bit nicer.

Fixes:

- Collecting stats was broken when "sessions" was disabled in the site settings.

- Use navigator.sendBeacon by default in count.js. This will allow using click
  events on things like PDF files and external URLs:

      <a href="file.pdf" data-goatcounter-event="file.pdf">
      <a href="http://example.com" data-goatcounter-event="ext-example.com">

- Sometimes the order of pages was wrong when using PostgreSQL.

- Few smaller bugfixes.

2022-11-15 v2.4.1
-----------------
- Fix regression that caused the charts for SQLite to be off.

2022-11-08 v2.4.0
-----------------
- Add a more fully-featured API that can also retrieve the dashboard statistics.
  See https://www.goatcounter.com/help/api for documentation.

  This is still as "v0" because some details may still change.

- Default API ratelimit is now 4 requests/second, rather than 4 requests/10
  seconds. You can use the `-ratelimit` flag to configure this.

- Can now also merge paths instead of just deleting them (the "Settings → Delete
  pageviews" tab was changed to "Manage pageviews").

- Add `goatcounter dashboard`, which uses the new API to display the dashboard
  in the terminal (only a basic non-interactive overview for now).

- Add a "Show fewer numbers" user setting; this is intended to still give a
  reasonably useful overview of what happens on your site but prevent an
  “obsession” over the exact number of visitors and stats.

- No longer store or display "pageviews": always store and display "visitors"
  instead.

  The visitor count is the only thing that's interesting in pretty much all
  cases; the "raw" pageviews are still stored for some future purposes (such as
  "time on page"), but are no longer stored in most other contexts.

- Add infrastructure for "dark mode".

  This is not yet enabled by default because all "dark mode" themes look "bad"
  on my eyes, and I'm not really sure what works well for people who do like it.

  So some help is needed here. See:
  https://github.com/arp242/goatcounter/issues/586#issuecomment-1287995673

2022-10-17 v2.3.0
-----------------
- Expand campaigns: the `utm_campaign` or `campaign` parameter now is tracked
  separately, and add a dashboard panel for campaigns. See:
  https://www.goatcounter.com/help/campaigns

  Old data isn't backfilled as this information wasn't stored.

- There are now binaries for Windows, macOS, {Free,Open}BSD, and illumos.

- WebSockets are now disabled by default, as it turned out a lot of people had
  trouble proxying them. You can enable it with `goatcounter serve -websocket`.

- Add `-dbconn` flag for `serve` to allow setting the maximum number of
  connections. The default is also lowered from 25 to 16 for PostgreSQL.

- Add `-store-every` flag to control how often to persist pageviews to the
  database.

- Add "Sites that can embed GoatCounter" setting to allow embedding GoatCounter
  in a frame.

- Add "Hide UI for public view" setting to allow hiding the UI chrome and
  display only the charts.

- Quite a few bugfixes and minor additions.

2022-02-16 v2.2.0
-----------------
- The database connection string changed; you now need to use `-db
  engine+connect string` rather than `engine://connect string`:

      -db sqlite+[sqlite connection string]
      -db postgresql+[sqlite connection string]

  Various aliases such a `sqlite3`, `postgres` also work.

  The previous "url-like" strings conflicted with PostgreSQL's URL connection
  strings, causing confusion.

  `://`-type strings without a `+` will be rewritten, but will issue a warning.

- GoatCounter can now collect language statistics as well, from the
  `Accept-Language` HTTP header. This is disabled by default, but can be enabled
  in the site settings.

- Charts are now drawn as a line chart by default; you can choose to use bar
  charts in the widget settings menu by selecting the "chart style" for the
  "Paths overview" and/or "Total site pageviews"

  Both charts are also completely reïmplemented by drawing on a canvas instead
  of aligning divs in a flexbox because rendering thousands of divs in a flexbox
  is actually fairly slow.

- The "View as text table" button in the header moved to the "Chart style"
  section mentioned above; this checkbox was added before the configurable
  dashboard feature, and especially now that you can set a chart style it makes
  more sense to set it there.

- Data is now sent over a WebSocket, rather than rendering everything. The
  upshot of this is that the perceived performance is better: it only needs to
  calculate the data that's initially visible, and it's okay to wait a bit for
  the data that's not. The downside is that you need JavaScript, but that was
  already the case to render the charts.

- There is a "server management" tab in the settings which allows viewing and
  editing some server internals. This page is only available to users with the
  (new) "server management" access.

  All sites with just one user have this user's permissions automatically
  "upgraded"; sites with more than one user since I don't know which user should
  have which permissions.

  To prevent updating users, you can use (*before* running migrations):

      % goatcounter db query "insert into version values ('2021-12-13-2-superuser')"

  To update an existing user, you can use:

      % goatcounter db update users -access superuser -find=martin@arp242.net

- Add `-ratelimit` flag to configure the built-in ratelimits (the default values
  are unchanged). See `goatcounter help serve` for details.

- New translations: Italian, Spanish (Chilean), Turkish.

2021-12-01 v2.1.0
-----------------
Aside from a number of small fixes and improvements, major changes include:

- Support for translations; see https://www.goatcounter.com/translating for
  details how to translate GoatCounter.

- The import path is now updated to use "zgo.at/goatcounter/v2" so that e.g. "go
  install zgo.at/goatcounter/v2" works. This should have been done with the 2.0
  release, but I didn't realize how this all worked.

- The visitor counter now supports the `start` and `end` parameters and the JSON
  endpoint returns `count` as well, to get the total pageview count.

- You can now make the dashboard viewable to anyone who has a secret token (e.g.
  https://mystats.example.com?access-token=5g4..)

This release requires Go 1.17 to build.

2021-04-13 v2.0.4
-----------------
- Deal with duplicate entries in the `user_agents` table in the migration
  instead of erroring out; mostly fixes a situation that could happen if you ran
  the broken migrations in 2.0.0 or 2.0.1.

2021-04-02 v2.0.3
-----------------
- Fix if you had already run the broken migrations in 2.0.0 or 2.0.1.

- Handle failures in `goatcounter import` a bit more gracefully.


2021-04-02 v2.0.2
-----------------
- Fix migration order.

- Don't display the expected "Memstore.Init: json: cannot unmarshal number /
  into Go struct field storedSession.paths of type int64" error log on startup;
  this got displayed once, but was a bit confusing.

- Display a message on startup after the first update to direct people towards
  the 2.0 release notes and "goatcounter reindex".


2021-03-29 v2.0.1
-----------------
- Fix migrations 🤦 They worked when they were written, but a bunch of things
  changed in GoatCounter and some older ones didn't run any more.

- Add `-test` flag to `goatcounter db migrate` to rollback a migration, so it's
  easier to test if migrations will run correctly without actually changing the
  database.


2021-03-29 v2.0.0
-----------------
The version is bumped to 2.0 because this contains a number of incompatible
changes: several CLI commands got changed, and it includes some large database
migrations – running them is a bit more complex than the standard migrations.

An overview of **incompatible** changes:

- There are some rather large changes to the database layout for better
  efficiency; this means:

  - Somewhat faster queries.
  - Greatly reduced disk space requirements for the database.
  - The Browsers, systems, size, and location stats are filtered if you enter
    something in "filter paths". Previously this always displayed the site
    totals.
  - "Purge path" now works as expected for all stats.
  - Easier to add new statistics in the future.

  To update:

  1. You **must** first update to 1.4.2 and run all migrations from that.
     **Updating from older versions directly to 2.0.0 will not work!**

  2. Run the migrations with `goatcounter serve -automigrate` or `goatcounter
     migrate`.

  3. You probably want to manually run `VACUUM` (or `VACUUM FULL` for
     PostgreSQL) after the migration to free up unused rows. This isn't strictly
     required, but frees up disk space, and removes some of the autovacuum
     pressure that will run in the background.

  4. Run `goatcounter reindex`.

  All of this may take a while if you've got a lot of data. For about 500,000
  pageviews it takes about 3 minutes on SQLite, but if you've got millions of
  pageviews it may take an hour or more.

  If you want to keep pageviews while this is running you can:

  1. Write it to a logfile from a proxy or temporary HTTP server and run
     `goatcounter import` on this after the migrations are done.

  2. Use `goatcounter buffer`.

- `goatcounter migrate` is now `goatcounter db migrate`. It also behaves a bit
  different:

  - `goatcounter db migrate pending` lists only pending migrations, and will use
    exit code 1 if there are any pending migrations.
  - `goatcounter db migrate list` lists all migrations, always exits with 0.

- If you use PostgreSQL you need PostgreSQL 12 or newer; this was already the
  case before and you could run in to some edge cases where things didn't work,
  but this is enforced now.

- The `none` value got removed from the `-tls` flag; use `tls=http` to not serve
  TLS. This was confusingly named as you can do `-tls=none,acme` to still
  generate ACME certificates, but `none` implies that nothing is done.

- `goatcounter create` is now `goatcounter db site create`, and some flags got
  changed:

  - `-domain` is now `-vhost`.
  - `-parent` is now `-link`.
  - `-email` is now `-user.email`.
  - `-password` is now `-user.password`.

- The `-port` flag for `goatcounter serve` is renamed to `-public-port`. This
  should clarify that this isn't the *listen* port, but just the port
  GoatCounter is publicly accessible on.

- The `-site` flag got removed from `goatcounter import`; you can now only use
  `-url` to set a GoatCounter site to import to. The automagic API key creation
  was more confusing than anything else.

  You can use `goatcounter db create apitoken` to create an API key from the CLI.

- If you build from source, the build flag to set the version changed from:

      -ldflags="-X main.version=..."

  to:

      -ldflags="-X zgo.at/goatcounter.Version=..."

- The CSV export format was increased to `2`; it now includes the parsed browser
  and system values in addition to the User-Agent header. Version 2.0 will not
  be able to import the older exports from version `1`.


**Other changes**:

- You can read pageviews from logfiles with the `goatcounter import` command;
  you can also send pageviews to goatcounter.com with this (you don’t need to
  self-host it). See `goatcounter help import` and the site code documentation
  for details.

- You can now create multiple users; before there was always a single one. You
  can add users in *Settings → Users*.

  As a consequence, "Site settings" and "User preferences" are now split in to
  two screens. The Settings button in the top-right now displays only site
  settings, and clicking on your email address in the top right displays user
  preferences, which every user can configure to their liking.

- You can now configure what's displayed on the dashboard, in what order, and
  configure some aspects of various "widgets". You can set it in *User
  preferences → Dashboard*. Some settings from the main settings page have moved
  there.

- You can save a default view for the dashboard. Instead of always loading the
  last week by default, you can now configure it to load the last month, or view
  by day, or anything you want really.

- You can choose which data to collect; you can disable collecting any
  User-Agent, location, Referrer information.

- Ability to record state/province/district in addition to country, so it
  records "US-TX" or "NL-NB" instead of "United States" or "Netherlands".

  This option can be disabled separately from recording the country (enabled by
  default) and you can set which countries to record it for (defaults to `US,
  RU, CH`).

  This requires specifying the path to a GeoIP City database, which isn't
  included since it's ~30M.

- There are now stable `count.v*.js` scripts that can use subresource integrity.
  See the integration code for a list and hashes.

- You can use `data-goatcounter-settings` on the `<script>` tag to load the
  settings (requires `count.v2.js` or newer).

- New `goatcounter buffer` command; this allows buffering of pageviews in case
  the backend is down, running migrations, etc. See `goatcounter help buffer`
  for more information.

- The database for PostgreSQL is now created automatically; you no longer need
  to do this manually.

- You can copy settings from a site to other sites in *Settings → Sites*.

- Add `goatcounter db` command; you can now edit and delete sites, users, and
  API keys from the CLI. The `create` and `migrate` commands are now merged in
  to this as subcommands.

- Add a `gcbench` utility for inserting random pageviews in a database; for
  testing and comparing performance. This might be useful for end-users too in
  some cases, for example to see how much performance difference SQLite and
  PostgreSQL will give you, or to test if frobbing with server settings makes a
  difference:

      $ go run ./cmd/gcbench -db sqlite://db/gcbench.sqlite3 -ndays=90 -npaths=100 -nhits=1_000_000
      $ go run ./cmd/gcbench -db postgresql://dbname=gcbench -ndays=90 -npaths=100 -nhits=1_000_000

  Right now it doesn't try super-hard to simulate read-world usage patterns: the
  distribution is always uniform, but it still gives a reasonably accurate
  indication for comparison purposes.

- Many other minor changes and improvements.

- For changes since RC1 see:
  https://github.com/arp242/goatcounter/compare/v2.0.0-rc1...v2.0.0


2020-11-10, v1.4.2
------------------

- Add a "visitor counter" image you can add to your website to display the
  number of visitors, similar to old-style counters back in the ’90s.

- Other than this, it's mostly contains a few minor bugfixes and the like. You
  can see a list of changes in the git log:
  https://github.com/arp242/goatcounter/compare/v1.4.1...v1.4.2


2020-09-04 v1.4.1
-----------------

A few small updates, fixes, and performance enhancements. Nothing major.

You can see a list of changes in the git log:
https://github.com/arp242/goatcounter/compare/v1.4.0...v1.4.1


2020-08-24 v1.4.0
-----------------

- **Change defaults for `-listen`**.

  The default for the `-listen` flag changed from `localhost:8081` to `:443`,
  which is probably a better and less confusing default for most people. There
  is also some more detailed docs available in `goatcounter help listen`.

- Set Cache-Control header for static files.

  The `Cache-Control` header is now set for static files. Since the "cache
  busting" happens based on the goatcounter version it's now recommended to set
  this if you're compiling GoatCounter yourself. See the updated README for
  instructions.

- Add multi-factor auth.

  TOTP-based multi-factor auth is now supported.

- Better export, export API, add import feature.

  You can now import the CSV exports, useful for migrating from self-hosted to
  goatcounter.com or vice versa, or for migrating from other systems. There is a
  web interface and a `goatcounter import` command.

  The export now supports a "pagination cursor", so you can export only rows you
  didn't previously export. This is especially useful with the new export API.
  which should make it easy to sync GoatCounter data with another external
  platform.

  See http://goatcounter.com/api for details on the export API.

- API for sending pageviews.

  Doing that with the regular `/count` is actually quite painful, as you quickly
  run in to ratelimits, need to set specific headers, etc. Adding an API
  endpoint for that makes things much easier.

- API for creating and editing additional sites.

- Some redesigns.

  The "Totals" is now placed below the Pages; I think it makes more sense there.
  The Y-axis for the totals is now also independent. There's also been a quite a
  few restylings.

- Add "text view" mode.

  View your data as a simple table without too much graphics; only the main
  "Pages" overview is implemented for now.

- Make it easier to skip your own views.

  Previously this required adding custom code, but now loading any page with
  `#toggle-goatcounter` added will enable/disable the GoatCounter tracking for
  that browser.

- Can now manage "additional sites" from self-hosted GoatCounter.

  This wasn't possible before for no other reason than laziness on my part 🙃

- public/count.js is now ISC licensed.

  Previously the EUPL applied, which is fairly restrictive and may prevent
  people from including/self-hosting the count.js script.

- Add `goatcounter db` command

  This is mostly useful for writing deploy scripts: `goatcounter db
  schema-sqlite` prints the SQLite schema, `schema-pgsql` prints the PostgreSQL
  schema, and `goatcounter db test` tests if the database exists.

- Session hashes are no longer persisted to the database

  This is kind of an internal change, but session hashes are now stored in
  memory only and never recorded to the database. There's no real reason to
  persistently store this information, and this is a (small) privacy/GDPR
  compliance improvement.


2020-06-01 v1.3.0
-----------------

Note: this release contains quite a few database migrations; they make take a
minute to run (depending on your table size), and you may want to run a `VACUUM`
afterwards.

- Remove email auth, replace `-auth` with `-email-from`.

  As mentioned in the 1.2 release the email authentication is now removed. You
  can still reset the password for old accounts.

  Since the email auth no longer exists the `-auth` parameter no longer makes
  sense. It's now replaced with `-email-from`, which can be set to just an email
  address.

  **Action required**: if you set the email address with `-auth` you'll have to
  change it to `-email-from`.

- Add OS stats, improve accuracy of browser stats.

  GoatCounter now tracks the OS/platform in addition to just the browser, and
  the accuracy of the browser stats should be improved.

  **Action required**: you'll need to populate the `system_stats` table:

      $ goatcounter reindex -table system_stats

  If you want to process all browser stats with the new logic too, then use this
  instead:

      $ goatcounter reindex -table system_stats,browser_stats

- Improve performance.

  Increase performance by quite a bit on large sites and time ranges.

- Remove the per-path scaling.

  Previously GoatCounter would scale the Y-axis different for every path in the
  dashboard, but this was more confusing than helpful. It's now always scaled to
  the maximum of all paths in the selected date range and filter, with a field
  to scale it lower on-demand if desired.

- Add totals overview.

  Add chart with totals for the selected date range and filter.

- Add `goatcounter.url()`, `goatcounter.filter()`.

  Adds two new methods to the `count.js` script so it's easier to use write own
  implementation. In addition the script will now issue a `console.warn()` if a
  request isn't being counted for some reason.


2020-05-18 v1.2.0
-----------------

There are a number of changes in 1.2, and a few which require a bit of action
when updating. Also see: https://www.arp242.net/goatcounter-1.2.html

- Password authentication.

  The email-based authentication has been deprecated in favour of password
  authentication.

  **Action required** Use the interface to set a password (you will get a
  notification about this). Email authentication still works, but will be
  removed in the next release, after which updating the password will be tricky.

- Unique visit tracking.

  GoatCounter now tracks unique visits (without using cookies).

  Technical documentation about the implementation is in
  [doc/sessions.md](doc/sessions.md).

  There are two ways to display the older stats:

  1. Do nothing; meaning that "visits" will be 0 for previous date ranges.

  2. Assign a new 'session' to every hit, so that unique visits will be the same
     as the number of pageviews.

  Doing option 2 is a potentially expensive database operation and not everyone
  may care so it's not done automatically; instructions for doing this are:

  - SQLite (do *not* do this on a running system; as far as I can tell there's
    no good way to get the next sequence ID while incrementing it):

        delete from sessions;
        update hits set session=id, first_visit=1;
        update sqlite_sequence set seq = (select max(session) from hits) where name='sessions';

  - PostgreSQL:

        update hits set session=nextval('sessions_id_seq'), first_visit=1;

  And then run `goatcounter reindex`.

- Improve bot detection.

  The bot detection is now improved; this will be applied to older pageviews in
  the database with a migration, but the cached statistics aren't updated
  automatically (as it can take a while for larger sites). Use the `reindex`
  command to fully update older pageviews (this is entirely optional).

- Track events.

  There is now better support to track events; see the updated documentation on
  the Site Code page for details.

- Better support for campaigns.

  There is now a "campaign parameters" setting; if the URL matches one of these
  parameters it will be set as the referrer (overriding the `Referer` header).

  The default is `utm_campaign, utm_source, ref`.

- Better export.

  The export was a quick feature added in the first version, but didn't scale
  well to larger sites with a lot of pageviews. This now works well for any
  number of pageviews.

- Many small improvements and bug fixes

  It's almost 2 months of work, and there have been many small changes, fixes,
  and improvements. I didn’t keep track of them all 😅


2020-03-27 v1.1.2
-----------------

- Fix small issue with the domain not showing correct in the site code 😅


2020-03-27 v1.1.1
-----------------

- Small bugfix release which fixes some small issues and improves a few small
  documentation issues. List of changes:
  https://github.com/arp242/goatcounter/compare/v1.1.0...v1.1.1

- The biggest change is that the `saas` command no longer works (and is no
  longer documented). It was only ever useful for hosting goatcounter.com, and
  has a number of assumptions and hard-coded values.

  If you're using `saas`, then you can migrate to `serve` by setting a custom
  domain (`sites.cname`) for all the sites. The `serve` command should work
  after that.


2020-03-18 v1.1.0
-----------------

- **Incompatible** Improve CLI UX.

  The entire CLI has been redone; the original wasn't very user-friendly for
  self-hosting. See `goatcounter help` for the full docs, but in brief:

      o Use "goatcounter serve" instead of just "goatcounter".
      o Create new sites with "goatcounter create".
      o Good support for TLS hosting and ACME certificates (see -tls flag).
      o Invert -prod to -dev (i.e. just drop -prod for production services, add -dev for development).
      o -smtp flag is no longer required.
      o -dbconnect                 →  -db
      o -pgsql                     →  -db postgresql://...
      o -staticdomain              →  no longer needed, but if you really want it you can
                                      append to domain: -domain example.com,static.example.com
      o -emailerrors               →  -errors mailto:...
      o goatcounter -migrate       →  goatcounter migrate
      o goatcounter -migrate auto  →  goatcounter serve -automigrate

- **Action required** Show top referrals.

  To populate the ref_stats and size_stats tables for older data, update first
  and then run:

      $ goatcounter reindex -confirm -table ref_stats
      $ goatcounter reindex -confirm -table size_stats

- Charts are displayed in local timezone.

- Add "IgnoreIPs" setting to ignore your own views.

- Link to paths by adding a new domain setting.

- Add configurable data retention.

- Allow configuring the thousands separator.

- Allow filtering pages in the dashboard.

- Improve the integration code.

- Allow sending emails without a relay.

- Add get_query() to count.js to get query parameter.

- Allow viewing the charts by day, instead of only by hour.


2020-01-13 v1.0.0
-----------------

Initial stable release.

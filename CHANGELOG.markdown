ChangeLog for GoatCounter
=========================

This list is not comprehensive, and only lists new features and major changes,
but not every minor bugfix.

The goatcounter.com service generally runs the latest master.

2020-05-18 v1.2.0
-----------------

There are a number of changes in 1.2, and a few which require a bit of action
when updating. Also see: https://www.arp242.net/goatcounter-1.2.html

- Password authentication (#232)

  The email-based authentication has been deprecated in favour of password
  authentication.

  **Action required** Use the interface to set a password (you will get a
  notification about this). Email authentication still works, but will be
  removed in the next release, after which updating the password will be tricky.

- Unique visit tracking (#212)

  GoatCounter now tracks unique visits (without using cookies).

  Technical documentation about the implementation is in
  [doc/sessions.markdown](doc/sessions.markdown).

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

- Improve bot detection (#219)

  The bot detection is now improved; this will be applied to older pageviews in
  the database with a migration, but the cached statistics aren't updated
  automatically (as it can take a while for larger sites). Use the `reindex`
  command to fully update older pageviews (this is entirely optional).

- Track events (#215)

  There is now better support to track events; see the updated documentation on
  the Site Code page for details.

- Better support for campaigns (#238)

  There is now a "campaign parameters" setting; if the URL matches one of these
  parameters it will be set as the referrer (overriding the `Referer` header).

  The default is `utm_campaign, utm_source, ref`.

- Better export (#221)

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
  https://github.com/zgoat/goatcounter/compare/v1.1.0...v1.1.1

- The biggest change is that the `saas` command no longer works (and is no
  longer documented). It was only ever useful for hosting goatcounter.com, and
  has a number of assumptions and hard-coded values.

  If you're using `saas`, then you can migrate to `serve` by setting a custom
  domain (`sites.cname`) for all the sites. The `serve` command should work
  after that.


2020-03-18 v1.1.0
-----------------

- **Incompatible** Improve CLI UX (#154, #173, #175, #181)

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

- **Action required** Show top referrals (#192)

  To populate the ref_stats and size_stats tables for older data, update first
  and then run:

      $ goatcounter reindex -confirm -table ref_stats
      $ goatcounter reindex -confirm -table size_stats

- Charts are displayed in local timezone (#155)

- Add "IgnoreIPs" setting to ignore your own views (#128)

- Link to paths by adding a new domain setting (#138)

- Add configurable data retention (#134)

- Allow configuring the thousands separator (#132)

- Allow filtering pages in the dashboard (#106)

- Improve the integration code (#122)

- Allow sending emails without a relay (#184)

- Add get_query() to count.js to get query parameter (#199)

- Allow viewing the charts by day, instead of only by hour (#169)


2020-01-13 v1.0.0
-----------------

Initial release.

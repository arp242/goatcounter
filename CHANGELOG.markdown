ChangeLog for GoatCounter
=========================

This list is not comprehensive, and only lists new features and major changes.

The goatcounter.com service generally runs the latest master.

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

- Paginate urls and add link to view more: same for refs
- Measure bounce rate/time on page?

- PostgreSQL
- Export data
- Front page

After v1
--------

- Signin from multiple browsers/locations?
- better ref filtering/parsing; possible get rid of ref_params?
- Multiple users, management, user preferences.
- Purge page (remove all occurances) and ignore page (don't add any more, could
  be done in JS?)
- Record status codes.
- Redo chart with SVG, quite large filesuze now, and the many DOM nodes aren't
  great for render performance either.
  "All time" is already 6.2M (251K compressed)
- Custom domain support: not hard but needs support for CSP etc. so needs to be
  a setting.
- Highlight referrers from own domain.
- Consider using another template engine?
  https://github.com/SlinSo/goTemplateBenchmark
- Remove # from refs? Or put in ref_params?
- Cache HTML for stats. We don't need to regen data from last month every time
  since it's always the same.
- Don't use double-quoted literals in SQL: https://sqlite.org/quirks.html#dblquote
- Record unique hits, we can do this by setting a short-lived cookie of 30 mins
  or so (this is what Fathom does).
- Browser stats could be better. Right now it's just a quick list (mostly for
  myself so I can see if people are using bots and such). We already import the
  github.com/mssola/user_agent package.
- The current day is always shown in full on the stats, so if it's 04:00 it'll
  show 20 more hours for the rest of the day.

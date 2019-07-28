- Paginate urls and add link to view more: same for refs
- Measure bounce rate/time on page?

- record User-Agent in seperate count db:

user_agent  varchar
number      int

- Front page
- PostgreSQL

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

- Front page
- PostgreSQL
- Cache HTML
- Remove # from refs? Or put in ref_params?
- Paginate urls and add link to view more: same for refs
- Measure bounce rate/time on page?
- Consider using another template engine?
  https://github.com/SlinSo/goTemplateBenchmark

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

For most people `{{.CountDomain}}/count.js` should be fine, but there are also
stable versions if you want to use subresource integrity (SRI). This will verify
the integrity of the script to ensure there are no changes, and browsers will
refuse to run it if there are.

You won’t get any updates, with this – the versioned script will always remain
the same. Any existing version of `count.js` is guaranteed to remain compatible,
but you may need to update it in the future for new features.

Latest
------
No changes

v5 (9 June 2025)
----------------
    <script data-goatcounter="{{.SiteURL}}/count"
            async src="//{{.CountDomain}}/count.v5.js"
            crossorigin="anonymous"
            integrity="sha384-atnOLvQb9t+jTSipvd75X2yginT4PjVbqDdlJAmxMm+wYElFmeR6EmLP5bYeoRVQ"></script>

- Use `<img>`-based fallback if `navigator.sendBeacon` fails, for example due to
  Content-Security-Policy errors.

- Expose `window.goatcounter.filter()` and `window.goatcounter.get_data()`.

- No longer check for `window.goatcounter.vars` and `window.counter` These were
  changed a week or so after the initial release over five years ago. AFAIK no
  one is using them. If you do, then use `window.goatcounter` (or
  `data-goatcounter-settings`) instead of `window.goatcounter.vars` and
  `data-goatcounter="url"` on the script tag instead of `window.counter`.

v4 (8 Dec 2023)
---------------
    <script data-goatcounter="{{.SiteURL}}/count"
            async src="//{{.CountDomain}}/count.v4.js"
            crossorigin="anonymous"
            integrity="sha384-nRw6qfbWyJha9LhsOtSb2YJDyZdKvvCFh0fJYlkquSFjUxp9FVNugbfy8q1jdxI+"></script>

- Use `navigator.sendBeacon` when available.

v3 (1 Dec 2021)
---------------
    <script data-goatcounter="{{.SiteURL}}/count"
            async src="//{{.CountDomain}}/count.v3.js"
            crossorigin="anonymous"
            integrity="sha384-QGgNMMRFTi8ul5kHJ+vXysPe8gySvSA/Y3rpXZiRLzKPIw8CWY+a3ObKmQsyDr+a"></script>

- Support `start` and `end` in the visitor counter.
- Update localhost filter to include `0.0.0.0`
- Tabs opened in the background didn't always get accounted for (see #487 for
  details).
- Remove the timeout; this was already increased from 3 to 10 seconds in v2.
  Tracking will now happen no matter how long it takes for the page to load.


v2 (11 Mar 2021)
----------------
    <script data-goatcounter="{{.SiteURL}}/count"
            async src="//{{.CountDomain}}/count.v2.js"
            crossorigin="anonymous"
            integrity="sha384-PeYXrhTyEaBBz91ANMgpSbfN1kjioQNPHNDbMvevUVLJoWrVEjDCpKb71TehNAlj"></script>

- Allow loading settings from `data-goatcounter-settings` on the `script` tag.
- Increase timeout from 3 seconds to 10 seconds.
- Add braces around `if` since some minifiers can't deal with "dangling else"
  well (the code is correct, it's the minifier that's broken).


v1 (25 Dec 2020)
----------------
    <script data-goatcounter="{{.SiteURL}}/count"
          async src="//{{.CountDomain}}/count.v1.js"
          crossorigin="anonymous"
          integrity="sha384-RD/1OXO6tEoPGqxhwMKSsVlE5Y1g/pv/Pf2ZOcsIONjNf1O+HPABMM4MmHd3l5x4"></script>

- Initial stable version.

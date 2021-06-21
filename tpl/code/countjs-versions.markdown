For most people `{{.CountDomain}}/count.js` should be fine, but there are also
stable versions if you want to use subresource integrity (SRI). This will verify
the integrity of the script to ensure there are no changes, and browsers will
refuse to run it if there are.

You won’t get any updates, with this – the versioned script will always remain
the same. Any existing version of `count.js` is guaranteed to remain compatible,
but you may need to update it in the future for new features.

Latest
------
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

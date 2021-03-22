Anyone can load your site with any query parameters, for example if you have
`http://example.com/page` then people can load this as
`http://example.com?page?hello`, and this will show as two different paths in
the dashboard: `/page` and `/page?hello`.

There is no way for GoatCounter to know if `?hello` is a meaningful query
parameter, as it may be used for navigation.

In practice, a lot of crawlers and scripts load the page with extra query
parameters. GoatCounter automatically strips te most common ones.

Using a canonical URL
---------------------
The easiest way to ensure that `/path` always shows up as `/path` is to add a
canonical URL in the `<head>`:

    <link rel="canonical" href="https://example.com/path">

The `href` can also be relative (e.g. `/path`).

This will only work if the canonical URL is on the same domain (with allowance
for the `www` subdomain); for example setting the canonical URL to:

    <link rel="canonical" href="https://my-other-site.com/path">

And then loading this page as `https://example.com/path` will mean GoatCounter
just ignore this value. This is because some people publish things on multiple
sites and then point at one as "canonical". This can be good for SEO, but not
good for tracking things in GoatCounter.

Be sure to understand the potential SEO effects before adding a canonical URL;
if you use query parameters for navigation then you probably *don’t* want to do
this.

Remove all query parameters
---------------------------
Alternatively you can send a custom `path` without the query parameters:

    <script>
        window.goatcounter = {
            path: location.pathname || '/',
        }
    </script>
    {{template "code" .}}

You can add individual query parameters with `goatcounter.get_query()`:

    <script>
        window.goatcounter = {
            path: function() {
                return location.pathname + '?page=' + (goatcounter.get_query('page')) || '/'),
            },
        }
    </script>
    {{template "code" .}}

Note this example uses a callback, since `goatcouner.get_query()` won't be
defined yet if we just used an object.

The the [JavaScript API](/code/js) page for more details JS API.

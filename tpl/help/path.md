Sometimes you want to send a different path to GoatCounter than what appears in
the browser's URL bar; removing one or more query parameters are a common
scenario.

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

Using data-goatcounter-settings
--------------------------------
You can use `data-goatcounter-settings` on the script tag to set the path; this
must be valid JSON:

    <script data-goatcounter="{{.SiteURL}}/count"
            data-goatcounter-settings='{"path": "/hello"}'
            async src="//zgo.at/count.js"></script>

You can also set the `title`, `referrer`, and `event` in here.

Using window.goatcounter
------------------------
Alternatively you can send a custom `path` by setting `window.goatcounter`
*before* the `count.js` script loads:

    <script>
        window.goatcounter = {
            path: location.pathname || '/',
        }
    </script>
    {{template "code" .}}

This is useful if you want some more complex logic, for example to add some
individual query parameters with `goatcounter.get_query()`:

    <script>
        window.goatcounter = {
            path: function() {
                return location.pathname + '?page=' + (goatcounter.get_query('page')) || '/'),
            },
        }
    </script>
    {{template "code" .}}

Note this example uses a callback, since `goatcounter.get_query()` won't be
defined yet if we just used an object.

See the [JavaScript API]({{.Base}}/code/js) page for more details JS API.

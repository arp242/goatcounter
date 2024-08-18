A few examples on how to modify various parameters in the [JavaScript
API]({{.Base}}/code/js). Also see [Control the path that's sent to
GoatCounter]({{.Base}}/code/path).

Custom path and referrer
------------------------
A basic example with some custom logic for `path`:

    <script>
        window.goatcounter = {
            // The passed value is the default.
            path: function(p) {
                // Don't track the home page.
                if (p === '/')
                    return null

                // Remove .html from all other page links.
                return p.replace(/\.html$/, '')
            },
        }
    </script>
    {{template "code" .}}


Setting the endpoint in JavaScript
----------------------------------
Normally GoatCounter gets the endpoint to send pageviews to from the
`data-goatcounter` attribute on the `<script>` tag, but in some cases you may
want to modify that in JavaScript; you can use `goatcounter.endpoint` for that.

For example, to send to different sites depending on the current hostname:

    <script>
        var code = '';
        switch (location.hostname) {
        case 'example.com':
            code = 'a'
            break
        case 'example.org':
            code = 'b'
            break
        default:
            code = 'c'
        }
        window.goatcounter = {
            endpoint: 'https://' + code + '.goatcounter.com/count',
        }
    </script>
    <script async src="//{{.CountDomain}}/count.js"></script>

Note that `data-goatcounter` will always override any `goatcounter.endpoint`, so
don't include it!

And remember to do this before the `count.js` script is loaded, or call
`window.goatcounter.count()` manually.

The main way to add GoatCounter to a site is using the `count.js` script; this
is by far the easiest way to integrate GoatCounter, but other options are
available too.

The script is located at http://gc.zgo.at/count.js; although you can [host it
somewhere else]({{.Base}}/code/countjs-host) if you want, and there are a
[stable versions]({{.Base}}/code/countjs-versions) that can use subresource
integrity.

This script is served unminified by design so it can be easily examined. It's
not very large (~3.2K), and minifying would reduce it by just ~1K so you're not
saving much.

The script exposes `window.goatcounter` with various settings and methods.

Settings
--------
The easiest way to set these is by setting the `data-goatcounter-settings`
attribute on the `<script>` tag, but you can also set the on
`window.goatcounter`. A few examples are listed below.

The following settings are supported:

| Setting       | Description                                                                                                  |
| :------       | :----------                                                                                                  |
| `no_onload`   | Don’t do anything on page load, for cases where you want to call `count()` manually. Also won’t bind events. |
| `no_events`   | Don’t bind events.                                                                                           |
| `allow_local` | Allow requests from local addresses (`localhost`, `192.168.0.0`, etc.) for testing the integration locally.  |
| `allow_frame` | Allow requests when the page is loaded in a frame or iframe.                                                 |
| `endpoint`    | Customize the endpoint for sending pageviews to (overrides the URL in `data-goatcounter`). Only useful if you have `no_onload`. |

For example, to allow requests from local sources with:
`data-goatcounter-settings`:

    <script data-goatcounter="{{.SiteURL}}/count"
            data-goatcounter-settings='{"allow_local": true}'
            async src="//static.goatcounter.localhost:8081/count.js"></script>

Setting it on the attribute prevents having to add a new `<script>` with inline
JS. Take care that this is valid JSON! You should see an error message in the
browser log if it's not. This will **override** anything that's already present
in `window.goatcounter`.

You can also use `window.goatcounter`; this is identical to the above:

    <script>
        // Make sure this is *before* you load the count.js script; otherwise
        // the pageview may get sent before this is loaded and this will just
        // overwrite the object.
        window.goatcounter = {allow_local: true}
    </script>
    {{template "code" .}}

Data parameters
---------------
You can customize the data sent to GoatCounter; the default value will be used
if the value is `null` or `undefined`, but *not* on empty string, `0`, or
anything else!

You can also use a callback: the default value is passed and the return value is
sent to the server. No pageview is sent at all if the callback for `path`
returns `null`.

| Variable   | Description                                                                                                                                        |
| :-------   | :----------                                                                                                                                        |
| `path`     | Page path (without domain) or event name. Default is the value of `<link rel="canonical">` if it exists, or `location.pathname + location.search`. |
| `title`    | Human-readable title. Default is `document.title`.                                                                                                 |
| `referrer` | Where the user came from; can be an URL (`https://example.com`) or any string (`June Newsletter`). Default is to use the `Referer` header.         |
| `event`    | Treat the `path` as an event, rather than a URL. Boolean.                                                                                          |

Like with the settings above, you can use both the `data-goatcounter-settings`
attribute and `window.goatcounter` object. For example, to always send `/hello`
as the path:

    <script data-goatcounter="{{.SiteURL}}/count"
            data-goatcounter-settings='{"path": "/hello"}'
            async src="//zgo.at/count.js"></script>

<!-- -->

    <script>
        window.goatcounter = {path: '/hello'}
    </script>
    {{template "code" .}}

A few more advanced examples are listed in [Change data before it's sent to
GoatCounter]({{.Base}}/code/modify).

Methods
-------

The following methods are available on `window.goatcounter` after it finished
loading. Since it's loaded as `async` by default it may not be loaded yet when
your script runs. Either remove the `async` or use a little `setInterval()`
callback:

    var t = setInterval(function() {
        if (!window.goatcounter || !window.goatcounter.count)
            return

        clearInterval(t)
        // Do stuff with goatcounter here.
    }, 100)

### `count(vars)`
Send a pageview or event to GoatCounter; the `vars` parameter is an object as
described in the *Data parameters* section above, and will be merged in to the
global `window.goatcounter`, if it exists.

### `url(vars)`
Get URL to send to the server; the `vars` parameter behaves as `count()`.

Note that you may want to use `filter()` to exclude prerender requests and
various other things.

### `filter()`
Determine if this request should be filtered; this returns a string with the
reason or `false`.

This will filter some bots, pre-render requests, frames (unless `allow_frame` is
set), and local requests (unless `allow_local` is set).

Example usage:

    var f = goatcounter.filter()
    if (f) {
        if (console && 'log' in console)
            console.warn('goatcounter: not counting because of: ' + f)
        return
    }

### `bind_events()`
Bind a click event to every element with `data-goatcounter-click`. Called on
page load unless `no_onload` or `no_events` is set. You may need to call this
manually if you insert elements after the page loads.

See [Events]({{.Base}}/code/events) for more details about events.

### `get_query(name)`
Get a single query parameter from the current page’s URL; returns `undefined` if
the parameter doesn’t exist. This is useful if you want to get the `referrer`
from the URL:

    <script>
        window.goatcounter = {
            referrer: function() {
                return goatcounter.get_query('ref') ||
                    goatcounter.get_query('utm_campaign') ||
                    goatcounter.get_query('utm_source') ||
                    document.referrer
            },
        }
    </script>
    {{template "code" .}}

Note there is also a *Campaign Parameters* setting, which is probably easier for
most people. This is just if you want to get the campaign on only some pages, or
want to do some more advanced filtering (such as only including your own
campaigns).

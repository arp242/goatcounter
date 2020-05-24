{{define "code"}}&lt;script data-goatcounter="{{.Site.URL}}/count"
        async src="//{{.CountDomain}}/count.js"&gt;&lt;/script&gt;{{end}}
<pre>{{template "code" .}}</pre>

{{if eq .Path "/code"}}

Or use one of the ready-made integrations:

- [WordPress](https://github.com/zgoat/goatcounter-wordpress)<br>
  Use `{{.Site.URL}}/count` as the endpoint in the WordPress GoatCounter settings.

- [Gatsby](https://www.npmjs.com/package/gatsby-plugin-goatcounter)

- [schlix](https://www.schlix.com/extensions/analytics/goatcounter.html)


Table of Contents
-----------------
{:.no_toc}
- TOC
{:toc}

Events
------
GoatCounter will automatically bind a click event on any element with the
`data-goatcounter-click` attribute; for example to track clicks to an external
link as `ext-example.com`:

    <a href="https://example.com" data-goatcounter-click="ext-example.com">Example</a>

The `name` or `id` attribute will be used if `data-goatcounter-click` is empty,
in that order.

You can use `data-goatcounter-title` and `data-goatcounter-referrer` to set the
title and/or referrer:

    <a href="https://example.com"
       data-goatcounter-click="ext-example.com"
       data-goatcounter-title="Example event"
       data-goatcounter-referrer="hello"
    >Example</a>

The regular `title` attribute or the element’s HTML (capped to 200 characters)
is used if `data-goatcounter-title` is empty. There is no default for the
referrer.

Content security policy
-----------------------
You’ll need to add the following if you use a `Content-Security-Policy`:

    script-src  https://{{.CountDomain}}
    img-src     {{.Site.URL}}/count

The `script-src` is needed to load the `count.js` script, and the `img-src` is
needed to send pageviews to GoatCounter (which are loaded with a “tracking
pixel”).

Customizing
-----------
Customisation is done with the `window.goatcounter` object; the following keys
are supported:

### Settings

{:class="reftable"}
| Setting       | Description                                                                                                 |
| :------       | :----------                                                                                                 |
| `no_onload`   | Don’t do anything on page load. If you want to call `count()` manually. Also won’t bind events.             |
| `no_events`   | Don’t bind click events.                                                                                    |
| `allow_local` | Allow requests from local addresses (`localhost`, `192.168.0.0`, etc.) for testing the integration locally. |
| `allow_frame` | Allow requests when the page is loaded in a frame or iframe. |
| `endpoint`    | Customize the endpoint for sending pageviews to; see [Setting the endpoint in JavaScript ](#setting-the-endpoint-in-javascript). |

### Data parameters
You can customize the data sent to GoatCounter; the default value will be used
if the value is `null` or `undefined`, but *not* on empty string, `0`, or
anything else!

The value can be a callback: the default value is passed and the return value is
sent to the server. Nothing is sent if the return value from the `path` callback
is `null`.

{:class="reftable"}
| Variable   | Description                                                                                                                                        |
| :-------   | :----------                                                                                                                                        |
| `path`     | Page path (without domain) or event name. Default is the value of `<link rel="canonical">` if it exists, or `location.pathname + location.search`. |
| `title`    | Human-readable title. Default is `document.title`.                                                                                                 |
| `referrer` | Where the user came from; can be an URL (`https://example.com`) or any string (`June Newsletter`). Default is to use the `Referer` header.         |
| `event`    | Treat the `path` as an event, rather than a URL. Boolean.                                                                                          |

### Methods

#### `count(vars)`
Send a pageview or event to GoatCounter; the `vars` parameter is an object as
described in the Data section above, and will be merged in to the global
`window.goatcounter`, taking precedence.

Be aware that the script is loaded with `async` by default, so `count()` may not
yet be available on click events and the like. Use `setInterval()` to wait until
it’s available:

    elem.addEventListener('click', function() {
        var t = setInterval(function() {
            if (window.goatcounter && window.goatcounter.count) {
                clearInterval(t)
                goatconter.count()
            }
        }, 100)
    })

The default implementation already handles this, and you only need to worry
about this if you call `count()` manually.

#### `bind_events()`
Bind a click event to every element with `data-goatcounter-click`. Called on
page load unless `no_onload` or `no_events` is set. You may need to call this
manually if you insert elements after the page loads.

#### `get_query(name)`
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

Examples
--------

### Load only on production
You can check `location.host` if you want to load GoatCounter only on
`production.com` and not `staging.com` or `development.com`; for example:

    <script>
        // Only load on production environment.
        if (window.location.host !== 'production.com')
            window.goatcounter = {no_onload: true}
    </script>
    {{template "code" .}}

Note that [request from localhost are already
ignored](https://github.com/zgoat/goatcounter/blob/9525be9/public/count.js#L69-L72).

### Skip own views
You can use the same technique as a client-side way to skip loading from your
own browser:

    <script>
        if (window.location.hash === '#skipgc')
            localStorage.setItem('skipgc', 't')
        window.goatcounter = {no_onload: localStorage.getItem('skipgc') === 't'}
    </script>
    {{template "code" .}}

You can also fill in your IP address in the settings, or (temporarily) block the
`{{.CountDomain}}` domain.

### Custom path and referrer
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

### Multiple domains
GoatCounter doesn’t store the domain a pageview belongs to; if you add
GoatCounter to several (sub)domain then there’s no way to distinguish between
requests to `a.example.com/path` and `b.example.com/path` as they’re both
recorded as `/path`.

This might be improved at some point in the future; the options right now are:

1. Create a new “additional site” for every domain; this is a completely
   separate site which inherits the user, login, plan, etc. You will need to use
   a different site code for every (sub)domain.

2. If you want everything in a single overview then you can add the domain to
   the path, instead of just sending the path:

       <script>
           window.goatcounter = {
               path: function(p) { return location.host + p }
           }
       </script>
       {{template "code" .}}

   For subdomains it it might be more useful to just add the first domain label
   instead of the full domain here, or perhaps just a short static string
   identifying the source.

### Ignore query parameters in path
The value of `<link rel="canonical">` will be used automatically, and is the
easiest way to ignore extraneous query parameters:

    <link rel="canonical" href="https://example.com/path.html">

The `href` can also be relative (e.g. `/path.html`. Be sure to understand the
potential SEO effects before adding a canonical URL; if you use query parameters
for navigation then you probably *don’t* want to do this.

Alternatively you can send a custom `path` without the query parameters:

    <script>
        window.goatcounter = {
            path: location.pathname || '/',
        }
    </script>
    {{template "code" .}}

You can add individual query parameters with `get_query()`:

    window.goatcounter = {
        path: (location.pathname + '?page=' + get_query('page')) || '/',
    }

### SPA
Custom `count()` example for hooking in to an SPA nagivating by `#`:

    <script>
        window.goatcounter = {no_onload: true}

        window.addEventListener('hashchange', function(e) {
            window.goatcounter.count({
                path: location.pathname + location.search + location.hash,
            })
        })
    </script>
    {{template "code" .}}

### Custom events
You can send an event by setting the `event` parameter to `true` in `count()`.
For example:

    $('#banana').on('click', function(e) {
        window.goatcounter.count({
            path:  'click-banana',
            title: 'Yellow curvy fruit',
            event: true,
        })
    })

Note that the `path` doubles as the event name. There is currently no real way
to record the path with the event, although you can send it as part of the event
name:

    window.goatcounter.count({
        path:  function(p) { 'click-banana-' + p },
        event: true,
    })

The callback will have the regular `path` passed to it, and you can add an event
name there; you can also use `window.location.pathname` directly; the biggest
difference with the passed value is that `<link rel="canonical">` is taken in to
account.

### Consent notice
It is my understanding that GoatCounter does not need GDPR consent notices, but
right no-one can be 100% sure, lacking case law and clarification from the
member states' regulatory agents. See [GDPR consent
notices](https://www.goatcounter.com/gdpr) for some more details.

If you want to add a consent notice, then a simple example might be:

    <script>
        (function() {
            // Consent already given
            if (localStorage.getItem('consent') === 't')
                return

            // Don't do anyting on load.
            window.goatcounter = {no_onload: true}

            // Create a simple banner.
            var agree = document.createElement('a')
            agree.innerHTML = 'Yeah, I agree'
            agree.style.position = 'fixed'
            agree.style.left = '0'
            agree.style.right = '0'
            agree.style.bottom = '0'
            agree.style.textAlign = 'center'
            agree.style.backgroundColor = 'pink'

            // Send the event on click.
            agree.addEventListener('click', function(e) {
                e.preventDefault()
                localStorage.setItem('consent', 't')
                agree.parentNode.removeChild(agree)

                window.goatcounter.count()       // Send pageview.
                window.goatcounter.bind_events() // If you use events.
            })

            document.body.appendChild(agree)
        })()
    </script>
    {{template "code" .}}



Advanced integrations
---------------------

### Image-based tracking without JavaScript
The endpoint returns a small 1×1 GIF image. A simple no-JS way would be to load
an image on your site:

    <img src="{{.Site.URL}}/count?p=/test-img">

This won’t allow recording the referrer or screen size, and may also increase
the number of bot requests (we do our best to filter this out, but it’s hard to
get all of them, since many spam scrapers and such disguise themselves as
regular browsers).

Wrap in a `<noscript>` tag to use this only for people without JavaScript.

### Tracking from backend middleware
You can call `GET {{.Site.URL}}/count` from anywhere, such as your app’s
middleware. It supports the following query parameters:

- `p` → `path`
- `e` → `event`
- `t` → `title`
- `r` → `referrer`
- `s` → screen size, as `x,y,scaling`.
- `q` → Query parameters, for getting the campaign.
- `b` → hint if this should be considered a bot; should be one of the
        [`JSBot*` constants from isbot][isbot]; note the backend may override
        this if it detects a bot using another method.
- `rnd` → can be used as a “cache buster” since browsers don’t always obey
          `Cache-Control`; ignored by the backend.

The `User-Agent` header and remote address are used for the browser and
location.

Calling it from the middleware will probably result in more bot requests, as
mentioned in the previous section.

[isbot]: https://github.com/zgoat/isbot/blob/master/isbot.go#L28

### Location of count.js and loading it locally
You can load the `count.js` script anywhere on your page, but it’s recommended
to load it just before the closing `</body>` tag if possible.

The reason for this is that downloading the `count.js` script will take up some
bandwidth which could be better used for the actual assets needed to render the
site. The script is quite small (about 2K), so it’s not a huge difference, but
might as well put it in the best location if possible. Just insert it in the
`<head>` or anywhere in the `<body>` if your CMS doesn’t have an option to add
it there.

You can also host the `count.js` script yourself, or include it in your page
directly inside `<script>` tags. You won’t get any new features or other
updates, but the `/count` endpoint is guaranteed to remain compatible so it
should never break (any future incompatible changes will be a different
endpoint, such as `/count/v2`).

Be sure to include the `data-goatcounter` attribute on the script tag or set
`goatcounter.endpoint` so GoatCounter knows where to send the pageviews to:

    <script data-goatcounter="{{.Site.URL}}/count">
        // [.. contents of count.js ..]
    </script>

    // or:

    <script>
        window.goatcounter = {endpoint: '{{.Site.URL}}/count'}

        // [.. contents of count.js ..]
    </script>

### Setting the endpoint in JavaScript
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

Note that `data-goatcounter` will always override any `goatcounter.endpoint`.

{{end}} {{/* if eq .Path "/settings" */}}

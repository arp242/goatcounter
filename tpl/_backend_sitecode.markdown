{{define "code"}}&lt;script data-goatcounter="{{.Site.URL}}/count"
        async src="//{{.CountDomain}}/count.js"&gt;&lt;/script&gt;{{end}}
<pre>{{template "code" .}}</pre>

{{if eq .Path "/code"}}

Integrations
------------
{:.no_toc}

<div style="text-align: center">
<label for="int-url">Endpoint</label><br>
<input type="text" value="{{.Site.URL}}/count" style="width: 28em"><br>
<span style="color: #999">You’ll need to copy this to the integration settings</span>

<style>
.integrations         { display: flex; flex-wrap: wrap; justify-content: center; margin-top: 1em; margin-bottom: 2em; }
.integrations a img   { float: left; }
.integrations a       { line-height: 40px; padding: 10px; width: 10em; margin: 1em; box-shadow: 0 0 4px #cdc8a4; }
.integrations a:hover { text-decoration: none; color: #00f; background-color: #f7f7f7; }
</style>

<div class="integrations">
<a href="https://github.com/zgoat/goatcounter-wordpress">
    <img width="40" height="40" src="{{.Static}}/int-logo/wp.png"> WordPress</a>
<a href="https://www.npmjs.com/package/gatsby-plugin-goatcounter">
    <img width="40" height="40" src="{{.Static}}/int-logo/gatsby.svg"> Gatsby</a>
<a href="https://www.schlix.com/extensions/analytics/goatcounter.html">
    <img width="40" height="40" src="{{.Static}}/int-logo/schlix.png"> schlix</a>
</div>
</div>

Visitor counter
---------------
{:.no_toc}

You can display a page's view count on your website by adding a HTML document or
image. The easiest way to do this is from the JavaScript integration:

    <script>
        // Append at the end of <body>; can use a CSS selector to append
        // somewhere else.
        // Be sure to call this *after* the count.js script is loaded.
        window.goatcounter.visit_count({append: 'body'})
    </script>

{{if .Site.ID}}
**Note**: you will need to enable “Allow adding visitor counts on your website”
in your <a href="/settings/main">site settings</a>; this defaults to
*off* to prevent accidental unintentional leaking of data.
{{else}}
**Note**: you will need to enable “Allow adding visitor counts on your website”
in your site settings; this defaults to *off* to prevent accidental
unintentional leaking of data.
{{end}}

See the <a href="#visitor-counter-customisation">detailed documentation</a> for more
options and customisations.


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

### `url(vars)`
Get URL to send to the server; the `vars` parameter behaves as `count()`.

Note that you may want to use `filter()` to exclude prerender requests and
various other things.

### `filter()`
Determine if this request should be filtered; this returns a string with the
reason or `false`.

This will filter pre-render requests, frames (unless `allow_frame` is set), and
local requests (unless `allow_local` is set).

Example usage:

    var f = goatcounter.filter()
    if (f) {
        if (console && 'log' in console)
            console.warn('goatcounter: not counting because of: ' + f)
        return
    }

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
There is a ‘Ignore IPs’ settings in your site’s settings (*Settings →
Tracking*). All requests from any IP address added here will be ignored.

You can also add `#toggle-goatcounter` to your site's URL to block your browser;
for example:

    https://example.com**#toggle-goatcounter**

If you filled in the domain in your settings then there should be a link there.
If you edit it in your URL bar you may have to reload the page with F5 for it to
work (you should get a popup).

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

1. Create a new site for every domain; this is a completely separate site which
   has the same user, login, plan, etc. You will need to use a different site
   code for every (sub)domain.

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

### Using navigator.sendBeacon

You can use [`navigator.sendBeacon()`][beacon] with GoatCounter, for example to
send events when someone closes a page:

    <script>
        if (goatcounter.filter())
            return
        navigator.sendBeacon(goatcounter.url({
            event: true,
            path: function(p) {
                return 'unload-' + p
            },
        }))
    </script>
    {{template "code" .}}

[beacon]: https://developer.mozilla.org/en-US/docs/Web/API/Navigator/sendBeacon

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


Visitor counter customisation
-----------------------------
The `goatcounter.visit_count()` function adds a ‘visitor counter’ to display the
number of pageviews for a path. The function accepts an object with the
following options:

{:class="reftable"}
| Setting       | Description                                                                                            |
| :------       | :----------                                                                                            |
| `append`      | HTML selector to append to, can use CSS selectors as accepted by `querySelector()`. Default is `body`. |
| `type`        | Type to add: `html`, `svg`, or `png`. Default is `html`.                                               |
| `path`        | Path to display; normally this is detected from the URL, but you can override.                         |
| `no_branding` | Don't display “by GoatCounter” branding; requires a paid account and has no effect on free accounts.   |
| `attr`        | HTML attributes to set or override for the element.                                                    |
| `style`       | Extra CSS styling for HTML or SVG.                                                                     |

The HTML variant is recommended for most people as it's the easiest to customize
with CSS. The SVG version can be customized to some degree with CSS as well, and
the PNG version is a fixed 200×80 image which can't be customized.

The default size is 200×80, or 200×60 if `no_branding` is added. You can
override the size by adding `width` and `height` in `attr`.

The special path `TOTAL` (case-sensitive, no leading `/`) can be used to display
the site totals.

The images are cached for 15 minutes, so new pageviews don’t show up right away.

#### Customisation
You can add the `style` option to customize the looks, this only works for HTML
and SVG.

Things you can style:

    div                 Div Wrapper; HTML only.
    #gcvc-border        Border rect; SVG only.
    #gcvc-for           "Views for this page" text.
    #gcvc-views         Number with views.
    #gcvc-by            “stats by GoatCounter” text.

For example, to get a dark colour scheme:

	goatcounter.visit_count({append: '#stats', style: `
		div { border-color: #fff; background-color: #222; color: #fff; }
   `})

Or the same for SVG:

	goatcounter.visit_count({append: '#stats', type: 'svg', style: `
		#gcvc-border { fill: #222; stroke: #fff; }
		#gcvc        { fill: #fff; }
   `})

#### Advanced
You don't need to use the JavaScript integration, you can also add an iframe or
image "directly"; the paths are in the form of:

    {{.Site.URL}}/counter/[PATH].[EXT]

The `[PATH]` is the full path, including a leading `/` and the `[EXT]` is the
`html`, `png`, `svg`, or `json` extension. For example
`{{.Site.URL}}/counter//.html` will display the view acount for `/`, and
`{{.Site.URL}}/counter//test.html.html` will display the view count for
`/test.html`.

There are two query parameters: `no_branding`, to disable to “stats by
GoatCounter” text, and `style` to insert custom styles.

#### JSON
The `.json` extension will return the pageview count in JSON; you can't use this
with a HTML tag but it can be used if you want to build your own counter in
JavaScript.

It returns an Object with one value: `count_unique`, which contains the unique
visitor count as a formatted string with thousands seperators.

A simple example usage:

	var r = new XMLHttpRequest();
	r.addEventListener('load', function() {
		document.querySelector('#stats').innerText = JSON.parse(this.responseText).count_unique
	})
	r.open('GET', '{{.Site.URL}}/counter/' + encodeURIComponent(location.pathname) + '.json')
	r.send()


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
You can use the `/api/v0/count` API endpoint to send pageviews from essentially
anywhere, such as your app's middleware.

The [API documentation](https://www.goatcounter.com/api) contains more
information and some examples.

### Location of count.js
You can load the `count.js` script anywhere on your page, but it’s recommended
to load it just before the closing `</body>` tag if possible.

The reason for this is that downloading the `count.js` script will take up some
bandwidth which could be better used for the actual assets needed to render the
site. The script is quite small (about 2K), so it’s not a huge difference, but
might as well put it in the best location if possible. Just insert it in the
`<head>` or anywhere in the `<body>` if your CMS doesn’t have an option to add
it there.

### Subresource integrity and versioning
For most people `{{.CountDomain}}/count.js` should be fine, but if you want you
can verify the integrity of the externally loaded script with SRI; currently
published versions:

- **v1** (25 Dec 2020, up to date with `count.js`):

      <script data-goatcounter="{{.Site.URL}}/count"
              async src="//{{.CountDomain}}/count.v1.js"
              crossorigin="anonymous"
              integrity="sha384-RD/1OXO6tEoPGqxhwMKSsVlE5Y1g/pv/Pf2ZOcsIONjNf1O+HPABMM4MmHd3l5x4"></script>

This will verify the integrity of the script to ensure there are no changes, and
browsers will refuse to run it if there are.

You won’t get any updates, with this – the versioned script will always remain
the same. Any existing version of `count.js` is guaranteed to remain compatible,
but you may need to update it in the future for new features.

### Self-hosting count.js
You can host the `count.js` script yourself, or include it in your page directly
inside `<script>` tags. You won’t get any new features or other updates, but the
`/count` endpoint is guaranteed to remain compatible so it should never break
(any future incompatible changes will be a different endpoint, such as
`/count/v2`).

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

Any existing version of `count.js` is guaranteed to remain compatible, but you
may need to update it in the future for new features.

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

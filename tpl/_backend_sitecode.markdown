{{define "code"}}&lt;script data-goatcounter="{{.Site.URL}}/count"
        async src="//{{.CountDomain}}/count.js"&gt;&lt;/script&gt;{{end}}
<pre>{{template "code" .}}</pre>

{{if eq .Path "/code"}}

Or use one of the ready-made integrations:
[Gatsby](https://www.npmjs.com/package/gatsby-plugin-goatcounter),
[schlix](https://www.schlix.com/extensions/analytics/goatcounter.html).

Events
------
<p>GoatCounter will automatically bind a click event on any element with the
<code>data-goatcounter-click</code> attribute; for example to track clicks to an
external link as <code>ext-example.com</code>:</p>

<pre>
&lt;a href="https://example.com" data-goatcounter-click="ext-example.com"&gt;Example&lt;/a&gt;
</pre>

<p>The <code>name</code>, <code>id</code>, or <code>href</code> attribute will
be used if <code>data-goatcounter-click</code> is empty, in that order.</p>

<p>You can use <code>data-goatcounter-title</code> and
<code>data-goatcounter-referral</code> to set the title and/or referral:</p>
<pre>
&lt;a href="https://example.com" data-goatcounter-click="ext-example.com"
   data-goatcounter-title="Example event"
   data-goatcounter-referral="hello"&gt;Example&lt;/a&gt;
</pre>

<p>The regular <code>title</code> attribute or the element’s HTML (capped to 200
characters) is used if <code>data-goatcounter-title</code> is empty. There is
no default for the referrer.

Content security policy
-----------------------
You’ll need the following if you use a `Content-Security-Policy`:

	script-src  https://{{.CountDomain}}
	img-src     {{.Site.URL}}/count

Customizing
-----------
Customisation is done with the `window.goatcounter` object; the following keys
are supported:

### Settings

{:class="reftable"}
| Setting       | Description                                                                                                 |
| :------       | :----------                                                                                                 |
| `no_onload`   | Don’t do anything on page load. If you want to call `count()` manually.                                     |
| `no_events`   | Don’t bind click events.                                                                                    |
| `allow_local` | Allow requests from local addresses (`localhost`, `192.168.0.0`, etc.) for testing the integration locally. |

### Data
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
Count an event; the `vars` parameter is an object as described in the Data
section above, and will be merged in to the global `window.goatcounter`, taking
precedence.

Be aware that the script is loaded with `async` by default, so `count` may not
yet be available on click events and the like. To solve this, use `setInterval`
to wait until it’s available:

	elem.addEventListener('click', function() {
		var t = setInterval(function() {
			if (window.goatcounter &amp;&amp; window.goatcounter.count) {
				clearInterval(t);
				goatconter.count();
			}
		}, 100);
	});

The default implementation already handles this, and you only need to worry
about this if you call `count()` manually.

#### `bind_events()`
Bind a click event to every element with `data-goatcounter-click`. Called on
page load unless `no_events` is set. You may need to call this manually if you
insert elements after the page loads.

#### `get_query(name)`
Get a single query parameter from the current page’s URL; returns `undefined` if
the parameter doesn’t exist. This is useful if you want to get the `referrer`
from the URL:

	<script>
		referrer: function() {
			return goatcounter.get_query('ref') || goatcounter.get_query('utm_source') || document.referrer;
		}
	};
	</script>
	{{template "code" .}}

Examples
--------

### Load only on production
You can check `location.host` if you want to load GoatCounter only on
`production.com` and not `staging.com` or `development.com`; for example:

	<script>
		// Only load on production environment.
		if (window.location.host !== 'production.com')
			window.goatcounter = {no_onload: true};
	</script>
	{{template "code" .}}

Note that [request from localhost are already
ignored](https://github.com/zgoat/goatcounter/blob/9525be9/public/count.js#L69-L72)

### Skip own views
You can use the same technique as a client-side way to skip loading from your
own browser:

	<script>
		if (window.location.hash === '#skipgc')
			localStorage.setItem('skipgc', 't');
		window.goatcounter = {no_onload: localStorage.getItem('skipgc') === 't'};
	</script>
	{{template "code" .}}

You can also fill in your IP address in the settings, or (temporarily) block the
`{{.CountDomain}}` domain.

### Custom path and referrer
A basic example with some custom logic for `path`:

	<script>
		window.goatcounter = {
			path: function(p) {
				// Don't track the home page.
				if (p === '/')
					return null;

				// Remove .html from all other page links.
				return p.replace(/\.html$/, '');
			},
		};
	</script>
	{{template "code" .}}

### Ignore query parameters in path
The value of `<link rel="canonical">` will be used automatically, and is the
easiest way to ignore extraneous query parameters:

	<link rel="canonical" href="https://example.com/path.html">

The `href` can also be relative (e.g. `/path.html`. Be sure to understand the
potential SEO effects before adding a canonical URL! If you use query parameters
for navigation then you probably *don’t* want it.

Alternatively you can send a custom `path` without the query
parameters:

	<script>
		window.goatcounter = {
			path: location.pathname || '/',
		};
	</script>
	{{template "code" .}}

### SPA
Custom `count()` example for hooking in to an SPA:

	<script>
		window.goatcounter = {no_onload: true};

		window.addEventListener('hashchange', function(e) {
			window.goatcounter.count({
				path: location.pathname + location.search + location.hash,
			});
		});
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
		});
	})

Note that the `path` doubles as the event name.

Advanced integrations
---------------------

### Image
The endpoint returns a small 1×1 GIF image. A simple no-JS way would be to load
an image on your site:

	<img src="{{.Site.URL}}/count?p=/test-img">

This won’t allow recording the referral or screen size though, and may also
increase the number of bot requests (although we do our best to filter this
out).

Wrap in a `<noscript>` tag to use this only for people without JavaScript.

### From middleware
You can call `GET {{.Site.URL}}/count` from anywhere, such as your app's
middleware. It supports the following query parameters:

- `p` → `path`
- `e` → `event`
- `t` → `title`
- `r` → `referrer`
- `s` → screen size, as `x,y,scaling`.
- `rnd` → can be used as a “cache buster” since browsers don’t always obey
  `Cache-Control`; ignored by the backend.

The `User-Agent` header and remote address are used for the browser and
location.

Calling it from the middleware or as will probably result in more bot requests.
GoatCounter does its best to filter this out, but it’s impossible to do this
100% reliably.

{{end}} {{/* if eq .Path "/settings" */}}

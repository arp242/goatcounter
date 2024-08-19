You can display a page's view count on your website by adding a HTML document or
image.

{{if .FromWWW}}
**Note**: you will need to enable “Allow adding visitor counts on your website”
in your site settings; this defaults to *off* to prevent unintentional leaking of data.
{{else}}
**Note**: you will need to enable “Allow adding visitor counts on your website”
in your <a href="{{.Base}}/settings/main#section-site">site settings</a>; this defaults to
*off* to prevent unintentional leaking of data.
{{end}}

The easiest way to do this is from the JavaScript integration:

    <script>
        // Append to the <body>; can use a CSS selector to append somewhere else.
        window.goatcounter.visit_count({append: 'body'})
    </script>
    {{template "code" .}}

You want to make sure the `count.js` script is loaded before calling this; for
example:

    <script>
        var t = setInterval(function() {
            if (window.goatcounter && window.goatcounter.visit_count) {
                clearInterval(t)
                window.goatcounter.visit_count({append: 'body'})
            }
        }, 100)
    </script>
    {{template "code" .}}

You can also remove the `async` attribute.

An example of how this looks with the default settings:

<div id="vc-example">
<div id="vc-html">HTML<br></div>
<div id="vc-svg">SVG<br></div>
<div id="vc-png">PNG<br></div>
</div>


Customisation
-------------
The `goatcounter.visit_count()` function accepts an object with the following
options:

| Setting       | Description                                                                                                       |
| :------       | :----------                                                                                                       |
| `append`      | HTML selector to append to, can use CSS selectors as accepted by `querySelector()`. Default is `body`.            |
| `type`        | Type to add: `html`, `svg`, or `png`. Default is `html`.                                                          |
| `path`        | Path to display; normally this is detected from the URL, but you can override it.                                 |
| `no_branding` | Don't display “by GoatCounter” branding                                                                           |
| `attr`        | HTML attributes to set or override for the element, only when `type` is `html`.                                   |
| `style`       | Extra CSS styling for HTML or SVG; only when `type` is `html` or `svg`.                                           |
| `start`       | Start date; default is to include everything. As `year-month-day` or `week`, `month`, `year` for this period ago. |
| `end`         | End date; default is to include everything. As `year-month-day`.                                                  |

The HTML variant is recommended for most people as it's the easiest to customize
with CSS. The SVG version can be customized to some degree with CSS as well, and
the PNG version is a fixed 200×80 image which can't be customized.

The default size is 200×80, or 200×60 if `no_branding` is added. You can
override the size by adding `width` and `height` in `attr`.

The special path `TOTAL` (case-sensitive, no leading `/`) can be used to display
the site totals.

The images are cached for 30 minutes, so new pageviews don’t show up right away.

### CSS
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

### Direct URLs
You don't need to use the JavaScript integration, you can also add an iframe or
image "directly"; the paths are in the form of:

    {{.SiteURL}}/counter/[PATH].[EXT]

- The `[PATH]` is the full path, including a leading `/`.
- The `[EXT]` is the `html`, `png`, `svg`, or `json` extension.

For example
`{{.SiteURL}}/counter//.html` will display the view account for `/`, and
`{{.SiteURL}}/counter//test.html.html` will display the view count for
`/test.html`.

There are four query parameters: `no_branding`, `style`, `start`, and `end`,
which correspond to the settings in the table.

### JSON
The `.json` extension will return the pageview count in JSON; you can't use this
with a HTML tag but it can be used if you want to build your own counter in
JavaScript.

It returns an Object with `count`, containing the total number of visitors
as a formatted string with thousands separators.

There is also a `count_unique` field for backwards compatibility; the value is
identical to `count`. This should not be used for new code.

A simple example usage:

    <div>Number of visitors: <div id="stats"></div></div>

    <script>
        var r = new XMLHttpRequest();
        r.addEventListener('load', function() {
            document.querySelector('#stats').innerText = JSON.parse(this.responseText).count
        })
        r.open('GET', '{{.SiteURL}}/counter/' + encodeURIComponent(location.pathname) + '.json')
        r.send()
    </script>

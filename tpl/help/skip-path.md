Nothing will be automatically sent if `window.goatcounter.no_onload` is set; the
easiest way to set this is from `data-goatcounter-settings` on the script tag:

    <script data-goatcounter="{{.SiteURL}}/count"
            data-goatcounter-settings='{"no_onload": true}'
            async src="//zgo.at/count.js"></script>

For static or server-side rendered sites this is usually the simplest approach.

---

You can also set this in JavaScript (*before* the script loads); for example to
automatically skip if the `<body>`'s class contains `goatcounter-skip`:

    <script>
        window.goatcounter = {
            no_onload: body.classList.contains('goatcounter-skip'),
        }
    </script>
    {{template "code" .}}

Or match against a list of paths:

    <script>
        ['/wp-admin.php', '^/feed/.*'].forEach((p) => {
            if (p === window.location.pathname || window.location.pathname.match(p))
                window.goatcounter = {no_onload: true}
        })
    </script>
    {{template "code" .}}

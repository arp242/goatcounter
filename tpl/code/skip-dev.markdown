Ignore IPs
----------
There is a ‘Ignore IPs’ settings in your site’s settings (*Settings →
Tracking*). All requests from any IP address added here will be ignored.

JavaScript
----------
Add `#toggle-goatcounter` to your site's URL to block your browser; for example:

    https://example.com#toggle-goatcounter

If you filled in the domain in your settings then there should be a link there.
If you edit it in your URL bar you may have to reload the page with F5 for it to
work (you should get a popup).

Skip loading staging/beta sites
-------------------------------
You can check `location.host` if you want to load GoatCounter only on
`production.com` and not `staging.com` or `development.com`; for example:

    <script>
        // Only load on production environment.
        if (window.location.host !== 'production.com')
            window.goatcounter = {no_onload: true}
    </script>
    {{template "code" .}}

Request from `localhost` and the most common private networks are [already
ignored][l] unless you add `allow_local`.

[l]: https://github.com/arp242/goatcounter/blob/9525be9/public/count.js#L69-L72

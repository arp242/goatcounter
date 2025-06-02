You can host the `count.js` script yourself, or include it in your page directly
inside `<script>` tags. You won’t get any new features or other updates, but the
`/count` endpoint is guaranteed to remain compatible so it should never break
(any future incompatible changes will be a different endpoint, such as
`/count/v2`).

To host `count.js` somewhere else just copy it from https://gc.zgo.at/count.js
and adjust the URL in `src`.

For convenience, the script is also published as an [npm package](https://www.npmjs.com/package/goatcounter). Once installed, it can be programmatically copied from `./node_modules/goatcounter/public/count.js` to a public webroot directory during your application's build process.

If you include it in the page's body then be sure to include the
`data-goatcounter` attribute on the script tag, or set `goatcounter.endpoint` so
GoatCounter knows where to send the pageviews to:

    <script data-goatcounter="{{.SiteURL}}/count">
        // [.. contents of count.js ..]
    </script>

or:

    <script>
        window.goatcounter = {endpoint: '{{.SiteURL}}/count'}

        // [.. contents of count.js ..]
    </script>

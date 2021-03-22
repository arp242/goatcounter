Getting started is pretty easy, just add the following JavaScript anywhere on the page:

<pre>{{template "code" .}}</pre>

Check if your adblocker is blocking GoatCounter if you don’t see any pageviews
({{.SiteDomain}} and/or gc.zgo.at domain).

Integrations
------------

<div style="text-align: center">
<label for="int-url">Endpoint</label><br>
<input type="text" value="{{.SiteURL}}/count" style="width: 28em"><br>
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


After setup
-----------

Here are some things you may want to look at after setting up the above:

- Make sure GoatCounter is allowed in the [Content-Security-Policy](/code/csp)
  if you're using it.

- If you're not seeing any pageviews then chances are your browser's adblocker
  is blocking it. Disable it and check again. It can take about 10 seconds for
  pageviews to appear, but this should never be longer.
  [Details](/code/adblock).

- You may want to consider adding a canonical link, for example:

        <link rel="canonical" href="https://example.com/path.html">

    See [Control the path that's sent to GoatCounter](/code/path) for more details.

- [Prevent tracking my own pageviews?](/code/skip-dev) documents some ways you
  can ignore your own pageviews from showing up in the dashboard.

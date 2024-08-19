GoatCounter doesn’t store the domain a pageview belongs to; if you add
GoatCounter to several (sub)domains then there’s no way to distinguish between
requests to `a.example.com/path` and `b.example.com/path` as they’re both
recorded as `/path`.

This might be improved at some point in the future; the options right now are:

1. Create a new site for every domain; this is a completely separate site which
   has the same user, login, etc. You will need to use a different site for
   every (sub)domain.

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


Also see [setting the endpoint in JavaScript]({{.Base}}/code/modify#setting-the-endpoint-in-javascript-4).

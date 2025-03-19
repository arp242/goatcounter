Sometimes it may be useful to embed GoatCounter in a frame; by default embedding
GoatCounter in a frame is disallowed, but in *Settings → Sites that can embed
GoatCounter* you can add a list of domains or URLs that are allowed to embed
GoatCounter.

You can add:

- A domain such as `example.com`, which will enable embedding on that entire
  domain for http and https on the standard ports (80 and 443).
- An URL such as `https://example.com:8000` will allow embedding only over
  https and on port 8000.
- An URL such as `example.com/path` will allow embedding on that URL.

You will still need to login, which will work inside a frame. If you have
"Dashboard viewable by" set to "logged in users or with secret token" then you
will need to add the token to the frame's `src`; for example:

    <iframe src="{{.SiteURL}}?access-token=TOKEN"></iframe>

### Hiding the user interface
You can hide the UI chrome ("sign in" button, footer, date selector) by adding
`hideui=1` in the URL:

For public view:

    <iframe src="{{.SiteURL}}?hideui=1"></iframe>

Or with an access token:

    <iframe src="{{.SiteURL}}?access-token=TOKEN&hideui=1"></iframe>

This also removes some of the padding, max-width, and background colour, making
it easier to embed things in an iframe. If you're logged in no UI is removed.

**This is not a security feature and people will still be able to get a
different view by hacking the URL.**

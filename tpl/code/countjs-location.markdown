You can load the `count.js` script anywhere on your page, but it’s recommended
to load it just before the closing `</body>` tag if possible.

The reason for this is that downloading the `count.js` script will take up some
bandwidth which could be better used for the actual assets needed to render the
site. The script is quite small (about 2K), so it’s not a huge difference, but
might as well put it in the best location if possible. Just insert it in the
`<head>` or anywhere in the `<body>` if your CMS doesn’t have an option to add
it there.

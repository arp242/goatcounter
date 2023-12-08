You can send data from logfiles with the `goatcounter import` command; for
example:

    $ export GOATCOUNTER_API_KEY=[..]
    $ goatcounter import -follow -format=combined -exclude=static \
      -site='{{.SiteURL}}' \
      /var/log/nginx/access_log

This will keep watching the file for changes and report new pageviews as they
come in. You can also batch import the data from logfiles by dropping the
`-follow` flag.

See `goatcounter help import` and `goatcounter help logfile` for more details.

The biggest advantage of this is that you won't need to add any JavaScript to
your site and that nothing will be blocked by adblockers, but there are a few
downsides as well:

- There will be more bot requests.
- Some data won't be available: screen sizes, page titles.
- It won't disambiguate to canonical paths from `<link rel="canonical">`; i.e.
  `/page` and `/page?x=y` will show up as two different paths.

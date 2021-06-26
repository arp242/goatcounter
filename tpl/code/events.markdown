GoatCounter will automatically bind a click event on any element with the
`data-goatcounter-click` attribute; for example to track clicks to an external
link as `ext-example.com`:

    <a href="https://example.com" data-goatcounter-click="ext-example.com">Example</a>

The `name` or `id` attribute will be used if `data-goatcounter-click` is empty,
in that order.

You can use `data-goatcounter-title` and `data-goatcounter-referrer` to set the
title and/or referrer:

    <a href="https://example.com"
       data-goatcounter-click="ext-example.com"
       data-goatcounter-title="Example event"
       data-goatcounter-referrer="hello"
    >Example</a>

The regular `title` attribute or the element's HTML (capped to 200 characters)
is used if `data-goatcounter-title` is empty. There is no default for the
referrer.

### Sending events from JavaScript
You can send an event by setting the `event` parameter to `true` in `count()`.
For example:

    $('#banana').on('click', function(e) {
        window.goatcounter.count({
            path:  'click-banana',
            title: 'Yellow curvy fruit',
            event: true,
        })
    })

The `path` doubles as the event name. This cannot have `/` as the first
character.

There is currently no real way to record the path with the event, although you
can send it as part of the event name:

    window.goatcounter.count({
        path:  function(p) { return 'click-banana-' + p },
        event: true,
    })

The callback will have the regular `path` passed to it, and you can add an event
name there; you can also use `window.location.pathname` directly; the biggest
difference with the passed value is that `<link rel="canonical">` is taken in to
account.

You can use [`navigator.sendBeacon()`][beacon] with GoatCounter, for example to
send events when someone closes a page:

    <script>
        document.addEventListener('visibilitychange', function(e) {
            if (document.visibilityState !== 'hidden')
                return

            if (goatcounter.filter())
                return
            navigator.sendBeacon(goatcounter.url({
                event: true,
                path: function(p) { return 'unload-' + p },
            }))
        })
    </script>
    {{template "code" .}}

[beacon]: https://developer.mozilla.org/en-US/docs/Web/API/Navigator/sendBeacon

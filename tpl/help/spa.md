Custom `count()` example for hooking in to an SPA nagivating by `#`:

    <script>
        window.goatcounter = {no_onload: true}

        window.addEventListener('hashchange', function(e) {
            window.goatcounter.count({
                path: location.pathname + location.search + location.hash,
            })
        })
    </script>
    {{template "code" .}}

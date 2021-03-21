It is my understanding that GoatCounter does not need GDPR consent notices, but
right no-one can be 100% sure, lacking case law and clarification from the
member states' regulatory agents. See [GDPR consent
notices](https://www.goatcounter.com/gdpr) for some more details.

If you want to add a consent notice, then a simple example might be:

    <script>
        (function() {
            // Consent already given
            if (localStorage.getItem('consent') === 't')
                return

            // Don't do anyting on load.
            window.goatcounter = {no_onload: true}

            // Create a simple banner.
            var agree = document.createElement('a')
            agree.innerHTML = 'Yeah, I agree'
            agree.style.position = 'fixed'
            agree.style.left = '0'
            agree.style.right = '0'
            agree.style.bottom = '0'
            agree.style.textAlign = 'center'
            agree.style.backgroundColor = 'pink'

            // Send the event on click.
            agree.addEventListener('click', function(e) {
                e.preventDefault()
                localStorage.setItem('consent', 't')
                agree.parentNode.removeChild(agree)

                window.goatcounter.count()       // Send pageview.
                window.goatcounter.bind_events() // If you use events.
            })

            document.body.appendChild(agree)
        })()
    </script>
    {{template "code" .}}

// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

(function() {
	var init = function() {
		setup_imgzoom()
		fill_code()
		fill_tz()
		setup_donate()
	}

	var setup_imgzoom = function() {
		var img = document.querySelectorAll('img.zoom');
		for (var i=0; i<img.length; i++) {
			img[i].addEventListener('click', function(e) { imgzoom(this); }, false);
		}
	};

	var fill_tz = function() {
		var tz = document.getElementById('timezone');
		if (!tz || !window.Intl || !window.Intl.DateTimeFormat)
			return;
		tz.value = Intl.DateTimeFormat().resolvedOptions().timeZone;
	};

	var fill_code = function() {
		var code = document.getElementById('code'),
			name = document.getElementById('name');
		if (!code || !name)
			return;

		// Don't set the code if the user modified it.
		var modified = false;
		code.addEventListener('change', function(e) {
			modified = true;
		}, false);

		name.addEventListener('blur', function() {
			// Remove protocol from URL.
			this.value = this.value.replace(/^https?:\/\//, '');

			if (modified && code.value.length > 0)
				return;

			code.value = this.value.
				replace(/^www\./, '').         // www.
				replace(/\./g, '_').           // . -> _
				replace(/[^a-zA-Z0-9_]/g, ''). // Remove anything else
				toLowerCase();
		}, false);

		code.addEventListener('blur', function() {
			this.value = this.value.toLowerCase();
		}, false);
	};

	// Parse all query parameters from string to {k: v} object.
	var split_query = function(s) {
		s = s.substr(s.indexOf('?') + 1);
		if (s.length === 0)
			return {};

		var split = s.split('&'),
			obj = {};
		for (var i = 0; i < split.length; i++) {
			var item = split[i].split('=');
			obj[item[0]] = decodeURIComponent(item[1]);
		}
		return obj;
	};

	var setup_donate = function() {
		var form = document.getElementById('donate-form')
		if (!form)
			return;

		var err = function(e) { document.getElementById('stripe-error').innerText = e }

		var query = split_query(location.search)
		if (query['return']) {
			if (query['return'] !== 'success')
				return err('Looks like there was an error in processing the payment :-(')
			form.innerHTML = '<p>Thank you for your donation!</p>'
			return;
		}

		form.addEventListener('submit', function(e) {
			e.preventDefault();

			if (typeof(Stripe) === 'undefined') {
				alert('Stripe JavaScript failed to load from "https://js.stripe.com/v3"; ' +
					'ensure this domain is allowed to load JavaScript and reload the page to try again.');
				return;
			}

			var q = {five: 5, ten: 10, twenty: 20, fourty: 40}[document.activeElement.value]
			if (!q) {
				q = parseInt(document.getElementById('quantity').value, 10);
				if (q % 5 !== 0)
					return err('Amount must be in multiples of 5')
			}

			Stripe(form.dataset.key).redirectToCheckout({
				items:      [{sku: form.dataset.sku, quantity: q / 5}],
				successUrl: location.origin + '/contribute?return=success#donate',
				cancelUrl:  location.origin + '/contribute?return=cancel#donate',
			}).then(function(result) {
				err(result.error ? result.error.message : '');
			});
		}, false)
	}

	if (document.readyState === 'complete')
		init();
	else
		window.addEventListener('load', init, false);
})();

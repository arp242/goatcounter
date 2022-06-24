// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

(function() {
	var init = function() {
		setup_imgzoom()
		fill_code()
		fill_tz()
		setup_donate()
		billing_faq()
	}

	var billing_faq = function() {
		var dt = document.querySelectorAll('#home-pricing-faq dt')
		for (var i=0; i<dt.length; i++) {
			dt[i].addEventListener('click', function(e) {
				var dd = e.target.nextElementSibling
				dd.classList.add('cbox')
				if (dd.style.height === 'auto') {
					dd.style.padding = '0'
					dd.style.height = '0'
					dd.style.marginBottom = '0'
				} else {
					dd.style.padding = '.3em 1em'
					dd.style.height = 'auto'
					dd.style.marginBottom = '1em'
				}
			})
		}
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

	if (document.readyState === 'complete')
		init();
	else
		window.addEventListener('load', init, false);
})();

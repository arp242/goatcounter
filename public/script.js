// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at eupl12.zgo.at

(function() {
	var init = function() {
		setup_imgzoom();
		fill_code();
	};

	var setup_imgzoom = function() {
		var img = document.querySelectorAll('img.zoom');
		for (var i=0; i<img.length; i++) {
			img[i].addEventListener('click', function(e) { imgzoom(this); }, false);
		}
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

	if (document.readyState === 'complete')
		init();
	else
		window.addEventListener('load', init, false);
})();

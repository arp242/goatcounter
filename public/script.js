// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

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
		// Don't set the code if the user modified it.
		var modified = false;
		$('#code').on('change', function() { modified = true; })

		$('#name').on('blur', function() {
			// Remove protocol from URL.
			$(this).val($(this).val().replace(/^https?:\/\//, ''));

			var code = $('#code')
			if (modified && code.val().length > 0)
				return;

			code.val($(this).val().
				replace(/^www\./, '').        // www.
				replace(/\.\w+$/, '').        // Remove tld
				replace(/\.co$/, '').         // .co.uk, .co.nz
				replace('.', '_').            // . -> _
				replace(/[^a-zA-Z0-9_]/, ''). // Remove anything else
				toLowerCase());
		});

		$('#code').on('blur', function() {
			$(this).val($(this).val().toLowerCase());
		})
	};

	if (document.readyState === 'complete')
		init();
	else
		window.addEventListener('load', init, false);
})();

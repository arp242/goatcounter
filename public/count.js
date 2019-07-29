(function() { 
	'use strict';

	var vars = window.vars || {};

	// Find canonical location of the current page.
	var get_location = function() {
		var loc = window.location,
			c = document.querySelector('link[rel="canonical"][href]');
		// Parse in a tag to a Location object (canonical URL may be relative).
		if (c) {
			var a = document.createElement('a');
			a.href = c.href;
			loc = a;
		}

		return {
			p: (vars.path     || (loc.pathname + loc.search) || '/'),
			r: (vars.referrer || document.referrer),
		};
	};

	// Convert parameters to urlencoded string, starting with a ?
	//
	// e.g. ?foo=bar&a=b
	var to_params = function(obj) {
		var p = [];
		for (var k in obj)
			p.push(encodeURIComponent(k) + '=' + encodeURIComponent(obj[k]));
		return '?' + p.join('&');
	};

	// Count a hit.
	var count = function() {
		// Don't track pages fetched with the browser's prefetch algorithm.
		// See https://github.com/usefathom/fathom/issues/13
		if ('visibilityState' in document && document.visibilityState === 'prerender') {
			return;
		}

		// Add image to send request.
		var img = document.createElement('img');
		img.setAttribute('alt', '');
		img.setAttribute('aria-hidden', 'true');
		img.src = window.counter + to_params(get_location());
		img.addEventListener('load', function() {
			document.body.removeChild(img)
		}, false);

		// Remove the image after 3s if the onload event is never fired.
		window.setTimeout(function() {
			if (!img.parentNode)
				return;
			img.src = ''; 
			document.body.removeChild(img)
		}, 3000);

		document.body.appendChild(img);  
	};

	if (document.body === null)
		document.addEventListener('DOMContentLoaded', function() { count(); }, false);
	else
		count();
})();

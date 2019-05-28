(function() { 
	'use strict';

	var vars = vars || {};

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

		var host = (vars.host || loc.hostname);
		return {
			//h: host,
			p: (vars.path     || (loc.pathname + loc.search) || '/'),
			r:  (vars.referrer || (document.referrer.indexOf(host) > -1 ? document.referrer : '') || ''),
		};
	};

	var to_params = function(obj) {
		var p = [];
		for (var k in obj)
			p.push(encodeURIComponent(k) + '=' + encodeURIComponent(obj[k]));
		return '?' + p.join('&');
	};

	var count = function() {
		var img = document.createElement('img');
		img.setAttribute('alt', '');
		img.setAttribute('aria-hidden', 'true');
		img.src = counter + to_params(get_location());
		img.addEventListener('load', function() {
			document.body.removeChild(img)
		}, false);

		// Remove the image if the onload event is never fired.
		window.setTimeout(function() {
			if (!img.parentNode)
				return;
			img.src = ''; 
			document.body.removeChild(img)
		}, 1500);

		console.log('append');
		document.body.appendChild(img);  
	};

	if (document.body === null)
		document.addEventListener('DOMContentLoaded', function() { count(); }, false);
	else
		count();
})();

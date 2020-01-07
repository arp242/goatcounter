// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// See /bin/proxy on how to test this locally.
(function() { 
	'use strict';

	var VARS = {};
	if (window.goatcounter)
		VARS = window.goatcounter.vars || {};
	else if (window.vars)  // TODO: temporary compatibility.
		VARS = window.vars || {};

	// Find canonical location of the current page.
	var get_location = function(count_vars) {
		var results = {
			p: count_vars.path     || VARS.path,
			r: count_vars.referrer || VARS.referrer,
			t: count_vars.title    || VARS.title,
			d: count_vars.domain   || VARS.domain,
		};

		var rcb, pcb, tcb, dcb;
		if (typeof(results.r) === 'function')
			rcb = results.r;
		if (typeof(results.p) === 'function')
			pcb = results.p;
		if (typeof(results.t) === 'function')
			tcb = results.t;
		if (typeof(results.d) === 'function')
			dcb = results.d;

		// Get referrer.
		if (results.r === null || results.r === undefined)
			results.r = document.referrer;
		if (rcb)
			results.r = rcb(results.r);

		// Get title.
		if (results.t === null || results.t === undefined)
			results.t = document.title;
		if (tcb)
			results.t = tcb(results.t);

		// Get domain.
		if (results.d === null || results.d === undefined)
			results.d = location.hostname;
		if (tcb)
			results.d = tcb(results.d);

		// Get path.
		if (results.p === null || results.p === undefined) {
			var loc = location,
				c = document.querySelector('link[rel="canonical"][href]');
			// Parse in a tag to a Location object (canonical URL may be relative).
			if (c) {
				var a = document.createElement('a');
				a.href = c.href;
				loc = a;
			}
			results.p = (loc.pathname + loc.search) || '/';
		}
		if (pcb)
			results.p = pcb(results.p);

		return results;
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
	var count = function(count_vars) {
		// Don't track pages fetched with the browser's prefetch algorithm.
		// See https://github.com/usefathom/fathom/issues/13
		if ('visibilityState' in document && document.visibilityState === 'prerender')
			return;

		// Don't track private networks.
		if (location.hostname.match(/localhost$/) ||
			location.hostname.match(/^(127\.|10\.|172\.16\.|192\.168\.)/))
				return;

		var loc = get_location(count_vars || {});
		loc.s = [window.screen.width, window.screen.height, (window.devicePixelRatio || 1)];

		// null returned from user callback.
		if (loc.p === null)
			return;

		// Add image to send request.
		var img = document.createElement('img');
		img.setAttribute('alt', '');
		img.setAttribute('aria-hidden', 'true');
		img.src = window.counter + to_params(loc);
		img.addEventListener('load', function() { document.body.removeChild(img) }, false);

		// Remove the image after 3s if the onload event is never triggered.
		setTimeout(function() {
			if (!img.parentNode)
				return;
			img.src = ''; 
			document.body.removeChild(img)
		}, 3000);

		document.body.appendChild(img);  
	};

	// Expose public API.
	if (!window.goatcounter)
		window.goatcounter = {};
	window.goatcounter.count = count;

	if (!VARS.no_onload) {
		if (document.body === null)
			document.addEventListener('DOMContentLoaded', function() { count(); }, false);
		else
			count();
	}
})();

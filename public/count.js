// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

// See /bin/proxy on how to test this locally.
(function() {
	'use strict';

	var dep = '';

	if (window.vars) {  // TODO: temporary compatibility.
		window.goatcounter = window.vars;
		dep += 'window.vars';
	}
	else if (window.goatcounter && window.goatcounter.vars) {
		window.goatcounter = window.goatcounter.vars;
		dep += 'window.goatcounter.vars';
	}
	else
		window.goatcounter = window.goatcounter || {};

	// Get all data we're going to send off to the counter endpoint.
	var get_data = function(count_vars) {
		var results = {
			p: count_vars.path     || goatcounter.path,
			r: count_vars.referrer || goatcounter.referrer,
			t: count_vars.title    || goatcounter.title,
		};
		if (count_vars.event || goatcounter.event)
			results.e = true;

		// Save callbacks.
		var rcb, pcb, tcb;
		if (typeof(results.r) === 'function') rcb = results.r;
		if (typeof(results.t) === 'function') tcb = results.t;
		if (typeof(results.p) === 'function') pcb = results.p;

		// Get the values unless explicitly given.
		if (is_empty(results.r)) results.r = document.referrer;
		if (is_empty(results.t)) results.t = document.title;
		if (is_empty(results.p)) {
			var loc = location,
				c = document.querySelector('link[rel="canonical"][href]');
			// Parse in a tag to a Location object (canonical URL may be relative).
			if (c) {
				loc = document.createElement('a');
				loc.href = c.href;
			}
			results.p = (loc.pathname + loc.search) || '/';
		}

		// Apply callbacks.
		if (rcb) results.r = rcb(results.r);
		if (tcb) results.t = tcb(results.t);
		if (pcb) results.p = pcb(results.p);

		return results;
	};

	// Check if a value is "empty" for the purpose of get_data().
	var is_empty = function(v) {
		return v === null || v === undefined || typeof(v) === 'function';
	}

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

		// Find the tag used to load this script.
		var script = document.querySelector('script[data-goatcounter]'),
			endpoint = window.counter;  // TODO: temporary compat.
		if (script)
			endpoint = script.dataset.goatcounter;

		// Don't track private networks.
		if (location.hostname.match(/localhost$/) ||
			location.hostname.match(/^(127\.|10\.|172\.16\.|192\.168\.)/))
				return;

		var data = get_data(count_vars || {});
		data.s = [window.screen.width, window.screen.height, (window.devicePixelRatio || 1)];
		if (dep !== '')
			data.dep = dep;

		// null returned from user callback.
		if (data.p === null)
			return;

		// Add image to send request.
		var img = document.createElement('img');
		img.setAttribute('alt', '');
		img.setAttribute('aria-hidden', 'true');
		img.src = endpoint + to_params(data);
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
	window.goatcounter.count = count;

	if (!goatcounter.no_onload) {
		if (document.body === null)
			document.addEventListener('DOMContentLoaded', function() { count(); }, false);
		else
			count();
	}
})();

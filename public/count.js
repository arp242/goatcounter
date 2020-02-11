// GoatCounter: https://www.goatcounter.com
'use strict';

(function() {
	window.goatcounter = (window.goatcounter && goatcounter.vars)  // Compatibility
		? goatcounter.vars : (window.goatcounter || {})

	// Get all data we're going to send off to the counter endpoint.
	var get_data = function(count_vars) {
		var data = {
			p: count_vars.path     || goatcounter.path,
			r: count_vars.referrer || goatcounter.referrer,
			t: count_vars.title    || goatcounter.title,
			e: !!(count_vars.event || goatcounter.event),
			s: [window.screen.width, window.screen.height, (window.devicePixelRatio || 1)],
		}

		var rcb, pcb, tcb  // Save callbacks so they can be applied after getting the defaults.
		if (typeof(data.r) === 'function') rcb = data.r
		if (typeof(data.t) === 'function') tcb = data.t
		if (typeof(data.p) === 'function') pcb = data.p

		if (is_empty(data.r)) data.r = document.referrer
		if (is_empty(data.t)) data.t = document.title
		if (is_empty(data.p)) {
			var loc = location,
			    c   = document.querySelector('link[rel="canonical"][href]')
			if (c) {  // May be relative.
				loc = document.createElement('a')
				loc.href = c.href
			}
			data.p = (loc.pathname + loc.search) || '/'
		}

		if (rcb) data.r = rcb(data.r)
		if (tcb) data.t = tcb(data.t)
		if (pcb) data.p = pcb(data.p)
		return data
	}

	// Check if a value is "empty" for the purpose of get_data().
	var is_empty = function(v) { return v === null || v === undefined || typeof(v) === 'function' }

	// Create urlencoded string, starting with a ?: "?foo=bar&a=b" or "?".
	var to_params = function(obj) {
		return '?' + Object.keys(obj).map(function(k) {
			return encodeURIComponent(k) + '=' + encodeURIComponent(obj[k])
		}).join('&')
	}

	// Count a hit.
	goatcounter.count = function(count_vars) {
		if (document.hidden || ('visibilityState' in document && document.visibilityState === 'prerender'))
			return
		if (!goatcounter.allow_local && location.hostname.match(/(localhost$|^127\.|^10\.|^172\.16\.|^192\.168\.)/))
			return

		var script   = document.querySelector('script[data-goatcounter]'),
		    endpoint = window.counter  // Compatibility
		if (script)
			endpoint = script.dataset.goatcounter

		var data = get_data(count_vars || {})
		if (data.p === null)  // null returned from user callback.
			return

		var img = document.createElement('img'),
		    rm  = function() { if (img && img.parentNode) img.parentNode.removeChild(img) }
		img.src = endpoint + to_params(data)
		img.style.float = 'right'  // Affect layout less.
		img.setAttribute('alt', '')
		img.setAttribute('aria-hidden', 'true')

		setTimeout(rm, 3000) // In case the onload isn't triggered.
		img.addEventListener('load', rm, false);
		document.body.appendChild(img)
	}

	if (!goatcounter.no_onload)
		if (document.readyState === 'loading')
			document.addEventListener('DOMContentLoaded', goatcounter.count, false)
		else
			goatcounter.count()
})();

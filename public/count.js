// GoatCounter: https://www.goatcounter.com
(function() {
	'use strict';

	if (window.goatcounter && window.goatcounter.vars)  // Compatibility
		window.goatcounter = window.goatcounter.vars
	else
		window.goatcounter = window.goatcounter || {}

	// Get all data we're going to send off to the counter endpoint.
	var get_data = function(vars) {
		var data = {
			p: (vars.path     === undefined ? goatcounter.path     : vars.path),
			r: (vars.referrer === undefined ? goatcounter.referrer : vars.referrer),
			t: (vars.title    === undefined ? goatcounter.title    : vars.title),
			e: !!(vars.event || goatcounter.event),
			s: [window.screen.width, window.screen.height, (window.devicePixelRatio || 1)],
		}

		var rcb, pcb, tcb  // Save callbacks to apply later.
		if (typeof(data.r) === 'function') rcb = data.r
		if (typeof(data.t) === 'function') tcb = data.t
		if (typeof(data.p) === 'function') pcb = data.p

		if (is_empty(data.r)) data.r = document.referrer
		if (is_empty(data.t)) data.t = document.title
		if (is_empty(data.p)) {
			var loc = location,
			    c = document.querySelector('link[rel="canonical"][href]')
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

	// Object to urlencoded string, starting with a ?.
	var to_params = function(obj) {
		var p = []
		for (var k in obj)
			if (obj[k] !== '' && obj[k] !== null && obj[k] !== undefined && obj[k] !== false)
				p.push(encodeURIComponent(k) + '=' + encodeURIComponent(obj[k]))
		return '?' + p.join('&')
	}

	// Count a hit.
	window.goatcounter.count = function(vars) {
		if ('visibilityState' in document && document.visibilityState === 'prerender')
			return
		if (!goatcounter.allow_local && location.hostname.match(/(localhost$|^127\.|^10\.|^172\.(1[6-9]|2[0-9]|3[0-1])\.|^192\.168\.)/))
			return

		var script   = document.querySelector('script[data-goatcounter]'),
		    endpoint = window.counter  // Compatibility
		if (script)
			endpoint = script.dataset.goatcounter

		var data = get_data(vars || {})
		if (data.p === null)  // null from user callback.
			return

		data.rnd = Math.random().toString(36).substr(2, 5)  // Browsers don't always listen to Cache-Control.

		var img = document.createElement('img'),
		    rm  = function() { if (img && img.parentNode) img.parentNode.removeChild(img) }
		img.src = endpoint + to_params(data)
		img.style.float = 'right'  // Affect layout less.
		img.setAttribute('alt', '')
		img.setAttribute('aria-hidden', 'true')

		setTimeout(rm, 3000)  // In case the onload isn't triggered.
		img.addEventListener('load', rm, false)
		document.body.appendChild(img)
	}

	// Get a query parameter.
	window.goatcounter.get_query = function(name) {
		var s = location.search.substr(1).split('&')
		for (var i = 0; i < s.length; i++)
			if (s[i].toLowerCase().indexOf(name.toLowerCase() + '=') === 0)
				return s[i].substr(name.length + 1)
	}

	// Track click events.
	window.goatcounter.bind_events = function() {
		document.querySelectorAll("*[data-goatcounter-click]").forEach(function(elem) {
			var send = function() {
				goatcounter.count({
					event:    true,
					path:     (elem.dataset.goatcounterClick || elem.name || elem.id || elem.href || ''),
					title:    (elem.dataset.goatcounterTitle || elem.title || (elem.innerHTML || '').substr(0, 200) || ''),
					referral: (elem.dataset.goatcounterReferral || ''),
				})
			}
			elem.addEventListener('click', send, false)
			elem.addEventListener('auxclick', send, false)  // Middle click.
		})
	}

	if (!goatcounter.no_events)
		goatcounter.bind_events()
	if (!goatcounter.no_onload)
		if (document.body === null)
			document.addEventListener('DOMContentLoaded', function() { goatcounter.count() }, false)
		else
			goatcounter.count()
})();

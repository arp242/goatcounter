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
			b: is_bot(),
			q: location.search,
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

	// See if this looks like a bot; there is some additional filtering on the
	// backend, but these properties can't be fetched from there.
	var is_bot = function() {
		// Headless browsers are probably a bot.
		var w = window, d = document
		if (w.callPhantom || w._phantom || w.phantom)
			return 150
		if (w.__nightmare)
			return 151
		if (d.__selenium_unwrapped || d.__webdriver_evaluate || d.__driver_evaluate)
			return 152
		if (navigator.webdriver)
			return 153
		return 0
	}

	// Object to urlencoded string, starting with a ?.
	var urlencode = function(obj) {
		var p = []
		for (var k in obj)
			if (obj[k] !== '' && obj[k] !== null && obj[k] !== undefined && obj[k] !== false)
				p.push(encodeURIComponent(k) + '=' + encodeURIComponent(obj[k]))
		return '?' + p.join('&')
	}

	// Get the endpoint to send requests to.
	var get_endpoint = function() {
		var s = document.querySelector('script[data-goatcounter]');
		if (s && s.dataset.goatcounter)
			return s.dataset.goatcounter
		return (goatcounter.endpoint || window.counter)  // counter is for compat; don't use.
	}

	// Filter some requests that we (probably) don't want to count.
	goatcounter.filter = function() {
		if ('visibilityState' in document && (document.visibilityState === 'prerender' || document.visibilityState === 'hidden'))
			return 'visibilityState'
		if (!goatcounter.allow_frame && location !== parent.location)
			return 'frame'
		if (!goatcounter.allow_local && location.hostname.match(/(localhost$|^127\.|^10\.|^172\.(1[6-9]|2[0-9]|3[0-1])\.|^192\.168\.)/))
			return 'local'
		if (localStorage.getItem('skipgc') === 't')
			return 'disabled with #toggle-goatcounter'
		return false
	}

	// Get URL to send to GoatCounter.
	window.goatcounter.url = function(vars) {
		var data = get_data(vars || {})
		if (data.p === null)  // null from user callback.
			return
		data.rnd = Math.random().toString(36).substr(2, 5)  // Browsers don't always listen to Cache-Control.

		var endpoint = get_endpoint()
		if (!endpoint) {
			if (console && 'warn' in console)
				console.warn('goatcounter: no endpoint found')
			return
		}

		return endpoint + urlencode(data)
	}

	// Count a hit.
	window.goatcounter.count = function(vars) {
		var f = goatcounter.filter()
		if (f) {
			if (console && 'log' in console)
				console.warn('goatcounter: not counting because of: ' + f)
			return
		}

		var url = goatcounter.url(vars)
		if (!url) {
			if (console && 'log' in console)
				console.warn('goatcounter: not counting because path callback returned null')
			return
		}

		var img = document.createElement('img')
		img.src = url
		img.style.float = 'right'  // Affect layout less.
		img.setAttribute('alt', '')
		img.setAttribute('aria-hidden', 'true')

		var rm = function() { if (img && img.parentNode) img.parentNode.removeChild(img) }
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
		if (!document.querySelectorAll)  // Just in case someone uses an ancient browser.
			return

		var send = function(elem) {
			return function() {
				goatcounter.count({
					event:    true,
					path:     (elem.dataset.goatcounterClick || elem.name || elem.id || ''),
					title:    (elem.dataset.goatcounterTitle || elem.title || (elem.innerHTML || '').substr(0, 200) || ''),
					referrer: (elem.dataset.goatcounterReferrer || elem.dataset.goatcounterReferral || ''),
				})
			}
		}

		Array.prototype.slice.call(document.querySelectorAll("*[data-goatcounter-click]")).forEach(function(elem) {
			if (elem.dataset.goatcounterBound)
				return
			var f = send(elem)
			elem.addEventListener('click', f, false)
			elem.addEventListener('auxclick', f, false)  // Middle click.
			elem.dataset.goatcounterBound = 'true'
		})
	}

	// Make it easy to skip your own views.
	if (location.hash === '#toggle-goatcounter')
		if (localStorage.getItem('skipgc') === 't') {
			localStorage.removeItem('skipgc', 't')
			alert('GoatCounter tracking is now ENABLED in this browser.')
		}
		else {
			localStorage.setItem('skipgc', 't')
			alert('GoatCounter tracking is now DISABLED in this browser until ' + location + ' is loaded again.')
		}

	if (!goatcounter.no_onload) {
		var go = function() {
			goatcounter.count()
			if (!goatcounter.no_events)
				goatcounter.bind_events()
		}

		if (document.body === null)
			document.addEventListener('DOMContentLoaded', function() { go() }, false)
		else
			go()
	}
})();

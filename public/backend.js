// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

;(function() {
	'use strict';

	$(document).ready(function() {
		window.I18N              = JSON.parse($('#js-i18n').text())
		window.USER_SETTINGS     = JSON.parse($('#js-settings').text())
		window.CSRF              = $('#js-settings').attr('data-csrf')
		window.TZ_OFFSET         = parseInt($('#js-settings').attr('data-offset'), 10) || 0
		window.SITE_FIRST_HIT_AT = $('#js-settings').attr('data-first-hit-at') * 1000
		window.USE_WEBSOCKET     = $('#js-settings').attr('data-websocket') === 'true'
		window.WEBSOCKET         = undefined
		if (!USER_SETTINGS.language)
			USER_SETTINGS.language = 'en'

		;[report_errors, bind_tooltip, bind_confirm, translate_calendar].forEach((f) => f.call())
		;[page_dashboard, page_settings_main, page_user_pref, page_user_dashboard, page_bosmang]
			.forEach((f) => document.body.id === f.name.replace(/_/g, '-') && f.call())
	})

	// Set up error reporting.
	var report_errors = function() {
		window.onerror = on_error
		$(document).on('ajaxError', function(e, xhr, settings, err) {
			if (settings.url === '/jserr')  // Just in case, otherwise we'll be stuck.
				return
			var msg = T("error/load-url", {url: settings.url, error: err})
			console.error(msg)
			on_error(`ajaxError: ${msg}`, settings.url)
			alert(msg)
		})
	}

	// Report an error.
	var on_error = function(msg, url, line, column, err) {
		// Don't log useless errors in Safari: https://bugs.webkit.org/show_bug.cgi?id=132945
		if (msg === 'Script error.' && navigator.vendor && navigator.vendor.indexOf('Apple') > -1)
			return

		// I don't what kind of shitty thing is spamming me with this, but I've
		// gotten a lot of them and I'm getting tired of it.
		if (msg.indexOf("document.getElementsByTagName('video')[0].webkitExitFullScreen"))
			return

		jQuery.ajax({
			url:    '/jserr',
			method: 'POST',
			data:    {msg: msg, url: url, line: line, column: column, stack: (err||{}).stack, ua: navigator.userAgent, loc: window.location+''},
		})
	}

	// Show confirmation on everything with data-confirm.
	var bind_confirm = function() {
		$('body').on('click submit', '[data-confirm]', function(e) {
			if (e.type === 'click' && $(this).is('form'))
				return
			if (!confirm($(this).attr('data-confirm')))
				e.preventDefault()
		})
	}

	// Show custom tooltip on everything with a title attribute.
	var bind_tooltip = function() {
		var tip = $('<div id="tooltip"></div>')

		var display = function(e, t) {
			if (t.is('.rlink') && t[0].offsetWidth >= t[0].scrollWidth)
				return

			tip.remove().html(t.attr('data-title')).css({left: e.pageX, top: (e.pageY + 20)})
			t.one('mouseleave', () => { tip.remove() })
			$('body').append(tip)
			if (tip.height() > 30)  // Move to left if there isn't enough space.
				tip.css('left', 0).css('left', e.pageX - tip.width() - 8)
		}

		$('body').on('mouseenter', '[data-title]', function(e) {
			var t = $(e.target).closest('[data-title]')
			display(e, t)
		})

		$('body').on('mouseenter', '[title]', function(e) {
			var t     = $(e.target).closest('[title]'),
				ev    = $(e.target).closest('tr').hasClass('event'),
				title = t.attr('title')

			t.attr('data-title', title).removeAttr('title')
			display(e, t)
		})
	}

	var page_settings_main = function() {
		// Add current IP address to ignore_ips.
		$('#add-ip').on('click', function(e) {
			e.preventDefault()

			jQuery.ajax({
				url:     '/settings/main/ip',
				success: function(data) {
					var input   = $('[name="settings.ignore_ips"]'),
						current = input.val().split(',').
							map(function(m) { return m.trim() }).
							filter(function(m) { return m !== '' })

					if (current.indexOf(data) > -1) {
						$('#add-ip').after('<span class="err">IP ' + data + ' is already in the list</span>')
						return
					}
					current.push(data)
					var set = current.join(', ')
					input.val(set).
						trigger('focus')[0].
						setSelectionRange(set.length, set.length)
				},
			})
		})

		// Generate random token.
		$('#rnd-secret').on('click', function(e) {
			e.preventDefault()
			$('#settings-secret').val(Array.from(window.crypto.getRandomValues(new Uint8Array(20)), (c) => c.toString(36)).join(''))
			$('#settings-secret').trigger('change')
		})

		// Show secret token.
		$('#settings-public').on('change', function(e) {
			$('#secret').css('display', $(this).val() === 'secret' ? 'block' : 'none')
			if ($('#settings-secret').val() === '')
				$('#rnd-secret').trigger('click')
		}).trigger('change')

		// Update redirect link.
		$('#settings-secret').on('change', function(e) {
			$('#secret-url').val(`${location.protocol}//${location.host}?access-token=${this.value}`)
		}).trigger('change')
	}

	var page_user_pref = function() {
		// Set the timezone based on the browser's timezone.
		$('#set-local-tz').on('click', function(e) {
			e.preventDefault()

			if (!window.Intl || !window.Intl.DateTimeFormat) {
				alert("Sorry, your browser doesn't support accurate timezone information :-(")
				return
			}

			var zone = Intl.DateTimeFormat().resolvedOptions().timeZone
			$(`#timezone [value$="${zone}"]`).prop('selected', true)
		})
	}

	var page_user_dashboard = function() {
		// Add new widget.
		$('.widget-add-new select').on('change', function(e) {
			e.preventDefault()
			if (this.selectedIndex === 0)
				return

			jQuery.ajax({
				url:     '/user/dashboard/widget/' + this.selectedOptions[0].value,
				success: function(data) {
					var i    = 1 + $('.index').toArray().map((e) => parseInt(e.value, 10)).sort().pop(),
						html = $(data.replace(/widgets([\[_])0([\]_])/g, `widgets$1${i}$2`))
					html.find('.index').val(i)
					$('.widget-add-new').before(html)

					var s = $('.widget-add-new select')
					s[0].selectedIndex = 0
					s.trigger('blur')
				},
			})
		})

		// Remove widget.
		$('#widget-settings').on('click', '.dashboard-rm', function(e) {
			e.preventDefault()
			$(this).closest('.widget').remove()
		})

		// Show settings
		$('#widget-settings').on('click', 'a.show-s', function(e) {
			e.preventDefault()
			var s = $(this).closest('.widget').find('.widget-settings')
			s.css('display', s.css('display') === 'none' ? 'block' : 'none')
		})
		// Show settings with errors.
		$('.widget-settings').each(function(i, w) {
			if ($(w).find('.err').length)
				$(w).css('display', 'block')
		})

		// Set of drag & drop.
		//
		// TODO: my iPhone selects text on dragging. I can't get it to stop doing
		// that no matter what; it always re-selects afterwards.
		// https://github.com/bevacqua/dragula/issues/306
		// ... okay?
		var w = $('#widget-settings')
		dragula(w.toArray(), {
			moves: (el, source, handle, sibling) => handle.className === 'drag-handle',
		}).on('drop', () => {
			$('#widget-settings .widget').each((i, el) => { $(el).find('.index').val(i) })
		})

		// Reset to defaults.
		w.find('.widgets-reset').on('click', function(e) {
			e.preventDefault()
			var f = $(this).closest('form')
			f.find('input[name="reset"]').val('true')
			f.trigger('submit')
		})
	}

	var page_bosmang = function() {
		$('table.sort th').on('click', function(e) {
			var th       = $(this),
				num_sort = th.is('.n'),
				col      = th.index(),
				tbody    = th.closest('table').find('>tbody'),
				rows     = Array.from(tbody.find('>tr')),
				to_i     = (i) => parseInt(i.replace(/[^0-9.]/g, ''), 10) || 0,
				is_sort  = th.attr('data-sort') === '1'

			if (num_sort)
				rows.sort((a, b) => to_i(a.children[col].innerText) < to_i(b.children[col].innerText))
			else
				rows.sort((a, b) => a.children[col].innerText.localeCompare(b.children[col].innerText))
			if (is_sort)
				rows.reverse()

			tbody.html('').html(rows)
			th.closest('table').find('th').attr('data-sort', '0')
			th.attr('data-sort', is_sort ? '0' : '1')
		})
	}
})();

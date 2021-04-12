// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

;(function() {
	'use strict';

	$(document).ready(function() {
		window.USER_SETTINGS     = JSON.parse($('#js-settings').text())
		window.CSRF              = $('#js-settings').attr('data-csrf')
		window.TZ_OFFSET         = parseInt($('#js-settings').attr('data-offset'), 10) || 0
		window.SITE_FIRST_HIT_AT = $('#js-settings').attr('data-first-hit-at') * 1000

		;[report_errors, bind_tooltip, bind_confirm].forEach((f) => f.call())
		;[page_dashboard, page_billing, page_settings_main, page_user_pref, page_user_dashboard, page_bosmang]
			.forEach((f) => document.body.id === f.name.replace(/_/g, '-') && f.call())
	})

	// Set up error reporting.
	var report_errors = function() {
		window.onerror = on_error
		$(document).on('ajaxError', function(e, xhr, settings, err) {
			if (settings.url === '/jserr')  // Just in case, otherwise we'll be stuck.
				return
			var msg = `Could not load ${settings.url}: ${err}`
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

			var pos = {left: e.pageX, top: (e.pageY + 20)}
			// Position on top for the chart-bar.
			if (t.closest('.chart-bar').length > 0 && t.closest('.chart-left, .chart-right').length === 0) {
				var x = t.offset().left
				pos = {
					left: (x + 8),
					top:  (t.parent().position().top),
				}
			}

			tip.remove().html(t.attr('data-title')).css(pos)

			t.one('mouseleave', () => { tip.remove() })
			$('body').append(tip)

			// Move to left of cursor if there isn't enough space.
			if (tip.height() > 30)
				tip.css('left', 0).css('left', pos.left - tip.width() - 8)
		}

		// Translucent hover effect; need a new div because the height isn't 100%
		var add_cursor = function(t) {
			if (t.closest('.chart-bar').length === 0 || t.is('#cursor') || t.closest('.chart-left, .chart-right').length > 0)
				return

			$('#cursor').remove()
			t.parent().append($('<span id="cursor"></span>').
				on('mouseleave', function() { $(this).remove() }).
				attr('title', t.attr('data-title')).
				css({width: t.width(), left: t.position().left}))
		}

		$('body').on('mouseenter', '[data-title]', function(e) {
			var t = $(e.target).closest('[data-title]')
			display(e, t)
			add_cursor(t)
		})

		$('body').on('mouseenter', '[title]', function(e) {
			var t     = $(e.target).closest('[title]'),
				ev    = $(e.target).closest('tr').hasClass('event'),
				title = t.attr('title')

			// Reformat the title in the chart.
			if (t.is('div') && t.closest('.chart-bar').length > 0) {
				if ($('.pages-list').hasClass('pages-list-daily')) {
					var [day, views, unique] = title.split('|')
					title = `${format_date(day)}`
				}
				else {
					var [day, start, end, views, unique] = title.split('|')
					title = `${format_date(day)} ${un24(start)} – ${un24(end)}`
				}

				title += !views ? ', future' : `, ${unique} ${ev ? 'unique clicks' : 'visits'}; <span class="views">${views} ${ev ? 'total clicks' : 'pageviews'}</span>`
			}
			t.attr('data-title', title).removeAttr('title')

			display(e, t)
			add_cursor(t)
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

	var page_billing = function() {
		// Pricing FAQ
		$('#home-pricing-faq dt').on('click', function(e) {
			var dd = $(e.target).next().addClass('cbox')
			if (dd[0].style.height === 'auto')
				dd.css({padding: '0', height: '0', marginBottom: '0'})
			else
				dd.css({padding: '.3em 1em', height: 'auto', marginBottom: '1em'})
		})

		// Extra pageviews.
		$('#allow_extra').on('change', function(e) {
			$('#extra-limit').css('display', this.checked ? '' : 'none')
			$('#max_extra').trigger('change')
		}).trigger('change')
		$('#max_extra').on('change', function(e) {
			var p  = $('#n-pageviews'),
				n  = parseInt($(this).val(), 10) * 50000,
				pv = {business: 500_000, businessplus: 1_000_000}[p.attr('data-plan')] || 100_000,
				pn = {personal: 'Personal', personalplus: 'Starter', business: 'Business', businessplus: 'Business plus'}[p.attr('data-plan')]

			var t = $('#allow_extra').is(':checked') ? 'There is no limit on the number of pageviews.' : ''
			if (n)
				t = `Your limit will be <strong>${format_int(pv+n)}</strong> pageviews (${format_int(pv)} from the ${pn} plan, plus ${format_int(n)} extra).`
			p.html(t)
		}).trigger('change')

		// Show/hide donation options.
		$('.plan input').on('change', function() {
			$('.free').css('display', $('input[name="plan"]:checked').val() === 'personal' ? '' : 'none')
		}).trigger('change')

		var form     = $('#billing-form'),
			nodonate = false
		form.find('button').on('click', function() { nodonate = this.id === 'nodonate' })

		// Create new Stripe subscription.
		form.on('submit', function(e) {
			e.preventDefault()

			if (typeof(Stripe) === 'undefined') {
				alert('Stripe JavaScript failed to load from "https://js.stripe.com/v3"; ' +
					'ensure this domain is allowed to load JavaScript and reload the page to try again.')
				return
			}

			var err      = function(e) { $('#stripe-error').text(e); },
				plan     = $('input[name="plan"]:checked').val(),
				quantity = (plan === 'personal' ? (parseInt($('#quantity').val(), 10) || 0) : 1)

			if (!plan)
				return alert('You need to select a plan')

			form.find('button[type="submit"]').attr('disabled', true).text('Redirecting...')
			jQuery.ajax({
				url:    '/billing/start',
				method: 'POST',
				data:    {csrf: CSRF, plan: plan, quantity: quantity, nodonate: nodonate},
				success: function(data) {
					if (data.no_stripe)
						return location.reload()
					Stripe(form.attr('data-key')).redirectToCheckout({sessionId: data.id}).
						then(function(result) { err(result.error ? result.error.message : '') })
				},
				error: function(xhr, settings, e) {
					err(err)
					on_error(`/billing/start: csrf: ${csrf}; plan: ${plan}; q: ${quantity}; xhr: ${xhr}`)
				},
				complete: function() {
					form.find('button[type="submit"]').attr('disabled', false).text('Continue')
				},
			})
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

		// Warn when setting high limit.
		$('.widget-pages').on('change', function(e) {
			var w = $(this),
				v = parseInt(w.find('input[name$="limit_pages"]').val(), 10)
			w.find('.warn.red').remove()

			if (v > 25)
				w.find('.widget-settings').prepend(
					'<span class="warn red">Loading many pages may be slow, especially on slower devices. Set it to something lower if you’re experiencing performance problems.<br><br></span>')
		}).trigger('change')
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

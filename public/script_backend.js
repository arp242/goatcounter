// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

(function() {
	'use strict';

	var SETTINGS     = {},
		CSRF         = '',
		TZ_OFFSET    = 0,
		SITE_CREATED = 0

	$(document).ready(function() {
		SETTINGS     = JSON.parse($('#js-settings').text())
		CSRF         = $('#js-settings').attr('data-csrf')
		TZ_OFFSET    = parseInt($('#js-settings').attr('data-offset'), 10) || 0
		SITE_CREATED = $('#js-settings').attr('data-created') * 1000

		;[report_errors, period_select, load_refs, tooltip, paginate_paths,
			paginate_refs, hchart_detail, settings_tabs, paginate_locations,
			billing_subscribe, setup_datepicker, filter_paths, add_ip, fill_tz,
			draw_chart, bind_scale, tsort, copy_pre, ref_pages,
		].forEach(function(f) { f.call() })
	});

	// Set up error reporting.
	var report_errors = function() {
		window.onerror = on_error;

		$(document).on('ajaxError', function(e, xhr, settings, err) {
			if (settings.url === '/jserr')  // Just in case, otherwise we'll be stuck.
				return;
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
			return;

		jQuery.ajax({
			url:    '/jserr',
			method: 'POST',
			data:    {msg: msg, url: url, line: line, column: column, stack: (err||{}).stack, ua: navigator.userAgent, loc: window.location+''},
		});
	}

	// Load pages for reference in Totals
	var ref_pages = function() {
		$('.count-list').on('click', '.pages-by-ref', function(e) {
			e.preventDefault()
			var btn = $(this),
				p   = btn.parent()

			if (p.find('.list-ref-pages').length > 0) {
				p.find('.list-ref-pages').remove()
				return
			}

			$('.list-ref-pages').remove()
			var done = paginate_button(btn, () => {
				jQuery.ajax({
					url: '/pages-by-ref',
					data: append_period({name: btn.text()}),
					success: function(data) {
						p.append(data.html)
						done()
					}
				})
			})
		})
	}

	// Add copy button to <pre>.
	var copy_pre = function() {
		$('.site-code pre').each((_, elem) => {
			var btn = $('<a href="#" class="pre-copy">📋 Copy</a>').on('click', (e) => {
				e.preventDefault()

				var i = $('<textarea />').val(elem.innerText).css('position', 'absolute').appendTo('body')
				i[0].select()
				i[0].setSelectionRange(0, elem.innerText.length)
				document.execCommand('copy')
				i.remove()
			})

			// Need relative positioned wrapper.
			var wrap = $('<div class="pre-copy-wrap">').html($(elem).clone())
			$(elem).replaceWith(wrap)
			wrap.prepend(btn)
		})
	}

	// Bind the Y-axis scale actions.
	var bind_scale = function() {
		var redraw = () => {
			if (get_current_scale() === get_original_scale())
				$('#scale').removeClass('value')
			else
				$('#scale').addClass('value')

			$('.count-list-pages').attr('data-scale', get_current_scale())
			$('.chart-bar').each((_, c) => { c.dataset.done = '' })
			draw_chart()
		}

		var t;
		$('#scale')
			.on('keydown', (e) => {
				if (e.keyCode === 13)
					e.preventDefault()
			})
			.on('input', (e) => {
				clearTimeout(t)
				t = setTimeout(redraw, 300)
			})

		$('#scale-half').on('click', (e) => {
			clearTimeout(t)
			e.preventDefault()
			$('#scale').val(Math.max(10, Math.ceil(parseInt(get_current_scale(), 10) / 2)))
			redraw()
		})

		$('#scale-reset').on('click', (e) => {
			clearTimeout(t)
			e.preventDefault()
			$('#scale').val(get_original_scale())
			redraw()
		})
	}

	// Replace the "height:" style with a background gradient and set the height
	// to 100%.
	//
	// This way you can still hover the entire height.
	var draw_chart = function() {
		var scale = parseInt(get_current_scale(), 10) / parseInt(get_original_scale(), 10)
		$('.chart-bar').each(function(i, chart) {
			if (chart.dataset.done === 't')
				return

			// Don't repaint/reflow on every bar update.
			chart.style.display = 'none'

			$(chart).find('>div').each(function(i, bar) {
				if (bar.dataset.h !== undefined)
					var h = bar.dataset.h
				else {
					var h = bar.style.height
					bar.dataset.h = h
					bar.style.height = '100%'
				}

				if (bar.className === 'f')
					return
				else if (h === '')
					bar.style.background = 'transparent'
				else {
					var hu = bar.dataset.u
					if (scale && scale !== 1) {
						h  = (parseInt(h, 10)  / scale) + '%'
						hu = (parseInt(hu, 10) / scale) + '%'
					}

					bar.style.background = `
						linear-gradient(to top,
						#9a15a4 0%,
						#9a15a4 ${hu},
						#ddd ${hu},
						#ddd ${h},
						transparent ${h},
						transparent 100%)`
				}
			})
			chart.dataset.done = 't'
			chart.style.display = 'flex'
		})
	}

	// Add current IP address to ignore_ips.
	var add_ip = function() {
		$('#add-ip').on('click', function(e) {
			e.preventDefault();

			jQuery.ajax({
				url:     '/ip',
				success: function(data) {
					var input   = $('[name="settings.ignore_ips"]'),
						current = input.val().split(',').
							map(function(m) { return m.trim() }).
							filter(function(m) { return m !== '' });

					if (current.indexOf(data) > -1) {
						$('#add-ip').after('<span class="err">IP ' + data + ' is already in the list</span>');
						return;
					}
					current.push(data);
					var set = current.join(', ');
					input.val(set).
						trigger('focus')[0].
						setSelectionRange(set.length, set.length);
				},
			});
		});
	};

	// Set the timezone based on the browser's timezone.
	var fill_tz = function() {
		$('#set-local-tz').on('click', function(e) {
			e.preventDefault();

			// It's hard to reliably get the TZ in JS without this; we can just
			// get the offset (-480) and perhaps parse the Date string to get
			// "WITA". Browser support is "good enough" to not bother with
			// complex workarounds: https://caniuse.com/#search=DateTimeFormat
			if (!window.Intl || !window.Intl.DateTimeFormat) {
				alert("Sorry, your browser doesn't support accurate timezone information :-(");
				return;
			}

			var zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
			$('#timezone [value$="' + zone + '"]').attr('selected', true);
		});
	};

	// Get the Y-axis scake.
	var get_original_scale = function(current) { return $('.count-list-pages').attr('data-max') }
	var get_current_scale  = function(current) { return $('#scale').val() }

	// Reload the path list when typing in the filter input, so the user won't
	// have to press "enter".
	var filter_paths = function() {
		highlight_filter($('#filter-paths').val());

		var t;
		$('#filter-paths').on('input', function(e) {
			clearTimeout(t);
			t = setTimeout(function() {
				var filter = $(e.target).val().trim()
				push_query('filter', filter)
				$('#filter-paths').toggleClass('value', filter !== '')

				var loading = $('<span class="loading"></span>')
				$(e.target).after(loading)
				jQuery.ajax({
					url:     '/pages',
					data:    append_period({
						filter: filter,
						daily:  $('#daily').is(':checked'),
						max:    get_original_scale(),
					}),
					success: function(data) {
						update_pages(data, true)
						loading.remove()
					},
				});
			}, 300);
		})

		// Don't submit form on enter.
		$('#filter-paths').on('keydown', function(e) {
			if (e.keyCode === 13)
				e.preventDefault()
		})
	};

	// Paginate the main path overview.
	var paginate_paths = function() {
		$('.pages-list .load-more').on('click', function(e) {
			e.preventDefault()
			var done = paginate_button($(this), () => {
				jQuery.ajax({
					url:  '/pages',
					data: append_period({
						filter:  $('#filter-paths').val(),
						daily:   $('#daily').is(':checked'),
						exclude: $('.count-list-pages >tbody >tr').toArray().map((e) => e.id).join(','),
						max:     get_original_scale(),
					}),
					success: function(data) {
						update_pages(data, false)
						done()
					},
				})
			})
		})
	};

	// Update the page list from ajax request on pagination/filter.
	var update_pages = function(data, from_filter) {
		if (from_filter) {
			$('.count-list-pages').attr('data-max', data.max)
			$('#scale').val(data.max)

			$('.pages-list .count-list-pages > tbody.totals').replaceWith(data.totals)
			$('.pages-list .count-list-pages > tbody.pages').html(data.rows)
		}
		else
			$('.pages-list .count-list-pages > tbody.pages').append(data.rows)

		highlight_filter($('#filter-paths').val())
		$('.pages-list .load-more').css('display', data.more ? 'inline' : 'none')

		var th = $('.pages-list .total-hits'),
		    td = $('.pages-list .total-display'),
			tu = $('.pages-list .total-unique'),
			ud = $('.pages-list .total-unique-display')
		if (from_filter) {
			th.text(format_int(data.total_hits));
			td.text(format_int(data.total_display));
			tu.text(format_int(data.total_unique));
			ud.text(format_int(data.total_unique_display));
		}
		else {
			td.each((_, t) => {
				$(t).text(format_int(parseInt($(t).text().replace(/[^0-9]/, ''), 10) + data.total_display));
			})
			ud.each((_, t) => {
				$(t).text(format_int(parseInt($(t).text().replace(/[^0-9]/, ''), 10) + data.total_unique_display));
			})
		}

		draw_chart()
	};

	// Highlight a filter pattern in the path and title.
	var highlight_filter = function(s) {
		if (s === '')
			return;
		$('.pages-list .count-list-pages > tbody.pages').find('.rlink, .page-title:not(.no-title)').each(function(_, elem) {
			if ($(elem).find('b').length)  // Don't apply twice after pagination
				return
			elem.innerHTML = elem.innerHTML.replace(new RegExp('' + quote_re(s) + '', 'gi'), '<b>$&</b>');
		});
	};

	// Setup datepicker fields.
	var setup_datepicker = function() {
		// Change to type="date" on mobile as that gives a better experience.
		//
		// Not done on *any* desktop OS as styling these fields with basic stuff
		// (like setting a cross-browser consistent height) is really hard and
		// fraught with all sort of idiocy.
		// They also don't really look all that great. Especially the Firefox
		// one looks pretty fucked.
		if (is_mobile()) {
			return $('#period-start, #period-end').
				attr('type', 'date').
				css('width', 'auto');  // Make sure there's room for UI chrome.
		}
		new Pikaday({field: $('#period-start')[0], toString: format_date_ymd, parse: get_date, firstDay: SETTINGS.sunday_starts_week ? 0 : 1});
		new Pikaday({field: $('#period-end')[0],   toString: format_date_ymd, parse: get_date, firstDay: SETTINGS.sunday_starts_week ? 0 : 1});
	};

	// Subscribe with Stripe.
	var billing_subscribe = function() {
		var form = $('#billing-form')
		if (!form.length)
			return;

		// Show/hide donation options.
		$('.plan input, .free input').on('change', function() {
			var personal = $('input[name="plan"]:checked').val() === 'personal',
				quantity = parseInt($('#quantity').val(), 10);

			$('.free').css('display', personal ? 'block' : 'none');
			$('.ask-cc').css('display', personal && quantity === 0 ? 'none' : 'block');
		}).trigger('change');

		form.on('submit', function(e) {
			e.preventDefault();

			if (typeof(Stripe) === 'undefined') {
				alert('Stripe JavaScript failed to load from "https://js.stripe.com/v3"; ' +
					'ensure this domain is allowed to load JavaScript and reload the page to try again.');
				return;
			}

			form.find('button').attr('disabled', true).text('Redirecting...');

			var err = function(e) { $('#stripe-error').text(e); },
				plan = $('input[name="plan"]:checked').val(),
				quantity = (plan === 'personal' ? (parseInt($('#quantity').val(), 10) || 0) : 1);
			jQuery.ajax({
				url:    '/billing/start',
				method: 'POST',
				data:    {csrf: CSRF, plan: plan, quantity: quantity},
				success: function(data) {
					if (data === '')
						return location.reload();
					Stripe(form.attr('data-key')).redirectToCheckout({sessionId: data.id}).
						then(function(result) { err(result.error ? result.error.message : ''); });
				},
				error: function(xhr, settings, e) {
					err(err);
				},
				complete: function() {
					form.find('button').attr('disabled', false).text('Continue');
				},
			});
		});
	};

	// Paginate the location chart.
	var paginate_locations = function() {
		$('.location-chart .show-all').on('click', function(e) {
			e.preventDefault();

			var bar = $(this).parent().find('.chart-hbar')
			var done = paginate_button($(this), () => {
				jQuery.ajax({
					url: '/locations',
					data: append_period(),
					success: function(data) {
						bar.html(data.html)
						done()
					},
				})
			})
		})
	}

	// Set up the tabbed navigation in the settings.
	var settings_tabs = function() {
		var nav = $('.tab-nav');
		if (!nav.length)
			return;

		var tabs   = '',
			active = location.hash.substr(5) || 'setting',
			valid  = !!( $('#' + active).length || $('#tab-' + active).length);

		// Link to a specific section for highlighting: set correct tab page.
		if (!$('#tab-' + active).is('h2') && location.hash.length > 2)
			active = $('#tab-' + active).closest('.tab-page').find('h2').attr('id')

		$('.page > div').each(function(i, elem) {
			var h2 = $(elem).find('h2')
			if (!h2.length)
				return

			var klass = '';
			// Only hide stuff if it's a tab we know about, to prevent nothing
			// being displayed.
			if (valid)
				if (h2.attr('id') !== active)
					$(elem).css('display', 'none')
				else
					klass = 'active'

			tabs += '<a class="' + klass + '" href="#tab-' + h2.attr('id') + '">' + h2.text() + '</a>'
		})
		nav.html(tabs);
		nav.on('click', 'a', function() {
			nav.find('a').removeClass('active')
			$(this).addClass('active')
		});

		$(window).on('hashchange', function() {
			if (location.hash === '')
				return;

			var tab = $('#' + location.hash.substr(5)).parent()
			if (!tab.length)
				return
			$('.page > div').css('display', 'none')
			tab.css('display', 'block')
		})
	};

	// Show details for the horizontal charts.
	var hchart_detail = function() {
		// Close on Esc or when clicking outside the hbar area.
		var close = function() {
			$('.hbar-detail').remove();
			$('.hbar-open').removeClass('hbar-open');
		};
		$(document.body).on('keydown', (e) => { if (e.keyCode === 27) close() });
		$(document.body).on('click',   (e) => { if ($(e.target).closest('.chart-hbar').length === 0) close() });

		$('.chart-hbar').on('click', 'a', function(e) {
			e.preventDefault();

			var btn  = $(this),
				bar  = btn.closest('.chart-hbar'),
				url  = bar.attr('data-detail'),
				name = btn.find('small').text();
			if (!url || !name || name === '(other)' || name === '(unknown)')
				return;

			btn.find('small').addClass('loading')
			jQuery.ajax({
				url: url,
				data: append_period({
					name:  name,
					total: $('.total-hits').text().replace(/[^\d]/, ''),
				}),
				success: function(data) {
					bar.parent().find('.hbar-detail').remove();
					btn.find('small').removeClass('loading')
					bar.addClass('hbar-open');

					var d = $('<div class="chart-hbar hbar-detail"></div>').css('min-height', (btn.position().top + btn.height()) + 'px').append(
						$('<div class="arrow"></div>').css('top', (btn.position().top + 6) + 'px'),
						data.html,
						$('<a href="#_" class="close">×</a>').on('click', function(e) {
							e.preventDefault();
							d.remove();
							bar.removeClass('hbar-open');
							btn.removeClass('active');
						}));

					bar.after(d);
				},
			});
		});
	};

	// Paginate the referrers.
	var paginate_refs = function() {
		$('.pages-list').on('click', '.load-more-refs', function(e) {
			e.preventDefault();

			var btn = $(this);
			var done = paginate_button(btn, () => {
				jQuery.ajax({
					url: '/refs',
					data: append_period({
						showrefs: btn.closest('tr').attr('id'),
						offset:   btn.prev().find('tr').length,
					}),
					success: function(data) {
						btn.prev().find('tbody').append($(data.rows).find('tr'));
						if (!data.more)
							btn.remove()
						done()
					},
				})
			})
		})
	};

	// Fill in start/end periods from buttons.
	var period_select = function() {
		$('.period-form-select input[type="checkbox"]').on('click', function(e) {
			$(this).closest('form').trigger('submit')
		})

		$('.period-form-select').on('click', 'button', function(e) {
			e.preventDefault();

			var start = new Date(), end = new Date();
			switch (this.value) {
				case 'day':       /* Do nothing */ break;
				case 'week':      start.setDate(start.getDate() - 7); break;
				case 'month':     start.setMonth(start.getMonth() - 1); break;
				case 'quarter':   start.setMonth(start.getMonth() - 3); break;
				case 'half-year': start.setMonth(start.getMonth() - 6); break;
				case 'year':      start.setFullYear(start.getFullYear() - 1); break;
				case 'week-cur':
					if (SETTINGS.sunday_starts_week)
						start.setDate(start.getDate() - start.getDay());
					else
						start.setDate(start.getDate() - start.getDay() + (start.getDay() ? 1 : -6));
					end = new Date(start.getFullYear(), start.getMonth(), start.getDate() + 6);
					break;
				case 'month-cur':
					start.setDate(1);
					end = new Date(end.getFullYear(), end.getMonth() + 1, 0);
					break;
			}

			$('#hl-period').val(this.value).attr('disabled', false);
			set_period(start, end);
		})

		$('.period-form-move').on('click', 'button', function(e) {
			e.preventDefault();
			var start = get_date($('#period-start').val()),
			    end   = get_date($('#period-end').val());

			// TODO: make something nicer than alert()s.
			if (this.value.substr(-2) === '-f' && end.getTime() > (new Date()).getTime())
				return alert('That would be in the future.')

			switch (this.value) {
				case 'week-b':    start.setDate(start.getDate() - 7);   end.setDate(end.getDate() - 7);   break;
				case 'month-b':   start.setMonth(start.getMonth() - 1); end.setMonth(end.getMonth() - 1); break;
				case 'week-f':    start.setDate(start.getDate() + 7);   end.setDate(end.getDate() + 7);   break;
				case 'month-f':   start.setMonth(start.getMonth() + 1); end.setMonth(end.getMonth() + 1); break;
			}
			if (start.getDate() === 1 && this.value.substr(0, 5) === 'month')
				end = new Date(start.getFullYear(), start.getMonth() + 1, 0)

			if (start > (new Date()).getTime())
				return alert('That would be in the future.')
			if (SITE_CREATED > end.getTime())
				return alert('That would be before the site’s creation; GoatCounter is not *that* good ;-)')

			set_period(start, end);
		})
	};

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list-pages').on('click', '.load-refs', function(e) {
			e.preventDefault()

			var params = split_query(location.search),
				link   = this,
				row    = $(this).closest('tr'),
				path   = row.attr('id'),
				close  = function() {
					var t = $(document.getElementById(params['showrefs']))
					t.removeClass('target')
					t.closest('tr').find('.refs').html('')
				}

			// Clicked on row that's already open, so close and stop. Don't
			// close anything yet if we're going to load another path, since
			// that gives a somewhat yanky effect (close, wait on xhr, open).
			if (params['showrefs'] === path) {
				close()
				return push_query('showrefs', null)
			}

			push_query('showrefs', path)

			var done = paginate_button($(link), () => {
				jQuery.ajax({
					url: '/refs' + link.search,
					success: function(data) {
						row.addClass('target')

						if (params['showrefs'])
							close()
						row.find('.refs').html(data.rows)
						if (data.more)
							row.find('.refs').append('<a href="#_", class="load-more-refs">load more</a>')
						done()
					},
				})
			})
		})
	}

	// Show custom tooltip on everything with a title attribute.
	var tooltip = function() {
		var tip = $('<div id="tooltip"></div>')

		var display = function(e, t) {
			if (t.is('.rlink') && t[0].offsetWidth >= t[0].scrollWidth)
				return

			var pos = {left: e.pageX, top: (e.pageY + 20)}
			if (t.closest('.chart-bar').length > 0) {
				var x = t.offset().left
				pos = {
					left: (x + 8),
					top:  (t.parent().position().top),
				}
			}

			tip.remove().html(t.attr('data-title')).css(pos)

			t.one('mouseleave', () => { tip.remove() })
			$('body').append(tip);

			// Move to left of cursor if there isn't enough space.
			if (tip.height() > 30)
				tip.css('left', 0).css('left', pos.left - tip.width() - 8)
		}

		// Translucent hover effect; need a new div because the height isn't
		// 100%
		var add_cursor = function(t) {
			if (t.closest('.chart-bar').length === 0 || t.is('#cursor'))
				return

			$('#cursor').remove()
			var cursor = $('<span id="cursor"></span>').
				on('mouseleave', () => { cursor.remove() }).
				attr('title', t.attr('data-title')).
				css({
					width: t.width(),
					left:  t.position().left - 3, // TODO: -3, why?
				})
				t.parent().append(cursor)
		}

		$('body').on('mouseenter', '[data-title]', function(e) {
			var t = $(e.target).closest('[data-title]')
			display(e, t)
			add_cursor(t)
		})

		$('body').on('mouseenter', '[title]', function(e) {
			var t     = $(e.target).closest('[title]'),
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

				title += !views ? ', future' : `, ${unique} visits; <span class="views">${views} pageviews</span>`
			}
			t.attr('data-title', title).removeAttr('title')

			display(e, t)
			add_cursor(t)
		})
	}

	// Prevent a button/link from working while an AJAX request is in progress;
	// otherwise smashing a "load more" button will load the same data twice.
	//
	// This also adds a subtle loading indicator after the link/button.
	//
	// TODO: this could be improved by queueing the requests, instead of
	// ignoring them.
	var paginate_button = function(btn, f) {
		if (btn.attr('data-working') === '1')
			return

		btn.attr('data-working', '1').addClass('loading')
		f.call(btn)
		return () => { btn.removeAttr('data-working').removeClass('loading') }
	}

	// Parse all query parameters from string to {k: v} object.
	var split_query = function(s) {
		s = s.substr(s.indexOf('?') + 1);
		if (s.length === 0)
			return {};

		var split = s.split('&'),
			obj = {};
		for (var i = 0; i < split.length; i++) {
			var item = split[i].split('=');
			obj[item[0]] = decodeURIComponent(item[1]);
		}
		return obj;
	};

	// Join query parameters from {k: v} object to href.
	var join_query = function(obj) {
		var s = [];
		for (var k in obj)
			s.push(k + '=' + encodeURIComponent(obj[k]));
		return (s.length === 0 ? '/' : ('?' + s.join('&')));
	};

	// Set one query parameter – leaving the others alone – and push to history.
	var push_query = function(k, v) {
		var params = split_query(location.search);
		if (v === null)
			delete params[k];
		else
			params[k] = v;
		history.pushState(null, '', join_query(params));
	}

	// Convert "23:45" to "11:45 pm".
	var un24 = function(t) {
		if (SETTINGS.twenty_four_hours)
			return t

		var hour = parseInt(t.substr(0, 2), 10);
		if (hour < 12)
			return t + ' am';
		else if (hour == 12)
			return t + ' pm';
		else
			return (hour - 12) + t.substr(2) + ' pm';
	};

	var months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug",
		          "Sep", "Oct", "Nov", "Dec"];
	var days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

	// Format a date according to user configuration.
	var format_date = function(date) {
		if (typeof(date) === 'string')
			date = get_date(date)

		var m = date.getMonth() + 1,
			d = date.getDate(),
			items = SETTINGS.date_format.split(/[-/\s]/),
			new_date = [];

		// Simple implementation of Go's time format. We only offer the current
		// formatters, so that's all we support:
		//   "2006-01-02"
		//   "02-01-2006"
		//   "01/02/06"
		//   "2 Jan 06"
		//   "Mon Jan 2 2006"
		for (var i = 0; i < items.length; i++) {
			switch (items[i]) {
				case '2006': new_date.push(date.getFullYear());                  break;
				case '06':   new_date.push((date.getFullYear() + '').substr(2)); break;
				case '01':   new_date.push(m >= 10 ? m : ('0' + m));             break;
				case '02':   new_date.push(d >= 10 ? d : ('0' + d));             break;
				case '2':    new_date.push(d);                                   break;
				case 'Jan':  new_date.push(months[date.getMonth()]);             break;
				case 'Mon':  new_date.push(days[date.getDay()]);                 break;
			}
		}

		var joiner = '-';
		if (SETTINGS.date_format.indexOf('/') > -1)
			joiner = '/';
		else if (SETTINGS.date_format.indexOf(' ') > -1)
			joiner = ' ';
		return new_date.join(joiner);
	};

	// Format a date as year-month-day.
	var format_date_ymd = function(date) {
		if (typeof(date) === 'string')
			return date;
		var m = date.getMonth() + 1,
			d = date.getDate();
		return date.getFullYear() + '-' +
			(m >= 10 ? m : ('0' + m)) + '-' +
			(d >= 10 ? d : ('0' + d));
	};

	// Format a number with a thousands separator. https://stackoverflow.com/a/2901298/660921
	var format_int = function(n) {
		return (n+'').replace(/\B(?=(\d{3})+(?!\d))/g, String.fromCharCode(SETTINGS.number_format));
	};

	// Create Date() object from year-month-day string.
	var get_date = function(str) {
		var s = str.split('-')
		return new Date(s[0], parseInt(s[1], 10) - 1, s[2])
	};

	// Append period-start and period-end values to the data object.
	var append_period = function(data) {
		data = data || {};
		data['period-start'] = $('#period-start').val();
		data['period-end']   = $('#period-end').val();
		return data;
	};

	// Set the start and end period and submit the form.
	var set_period = function(start, end) {
		if (TZ_OFFSET) {
			var offset = (start.getTimezoneOffset() + TZ_OFFSET) / 60;
			start.setHours(start.getHours() + offset);
			end.setHours(end.getHours() + offset);
		}

		$('#period-start').val(format_date_ymd(start));
		$('#period-end').val(format_date_ymd(end));
		$('#period-form').trigger('submit');
	};

	// Check if this is a mobile browser. Probably not 100% reliable.
	var is_mobile = function() {
		if (navigator.userAgent.match(/Mobile/i))
			return true;
		return window.innerWidth <= 800 && window.innerHeight <= 600;
	};

	// Quote special regexp characters. https://locutus.io/php/pcre/preg_quote/
	var quote_re = function(s) {
		return s.replace(new RegExp('[.\\\\+*?\\[\\^\\]$(){}=!<>|:\\-]', 'g'), '\\$&');
	};

	var tsort = function() {
		$('table.sort th').on('click', function(e) {
			var th       = $(this),
				num_sort = th.is('.n'),
				col      = th.index(),
				tbody    = th.closest('table').find('>tbody'),
				rows     = Array.from(tbody.find('>tr')),
				to_i     = (i) => parseInt(i.replace(/,/g, ''), 10),
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

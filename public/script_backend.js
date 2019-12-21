// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

(function() {
	'use strict';

	var SETTINGS = {};

	$(document).ready(function() {
		SETTINGS = JSON.parse($('#settings').html());

		$(document).ajaxError(function(e, xhr, settings, err) {
			var msg = 'Could not load ' + settings.url + ': ' + err;
			console.error(msg);
			alert(msg);
		});

		[period_select, drag_timeframe, load_refs, chart_hover, paginate_paths,
			paginate_refs, browser_detail, settings_tabs, paginate_locations,
			billing_subscribe,
		].forEach(function(f) { f.call(); });
	});

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
			form.find('button').attr('disabled', true).text('Redirecting...');

			var err = function(e) { $('#stripe-error').text(e); },
				plan = $('input[name="plan"]:checked').val(),
				quantity = (plan === 'personal' ? (parseInt($('#quantity').val(), 10) || 0) : 1);
			jQuery.ajax({
				url:    '/billing/start',
				method: 'POST',
				data:    {csrf: $('#csrf').val(), plan: plan, quantity: quantity},
				success: function(data) {
					if (data === '')
						return window.location.reload();
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
			jQuery.ajax({
				url: '/locations',
				data: append_period(),
				success: function(data) { bar.html(data.html); },
			});
		});
	};

	// Set up the tabbed navigation in the settings.
	var settings_tabs = function() {
		var nav = $('.tab-nav');
		if (!nav.length)
			return;

		var tabs = '',
			active = window.location.hash.substr(5) || 'setting',
			valid = !!$('#' + active).length;
		$('.page > div').each(function(i, elem) {
			var h2 = $(elem).find('h2');
			if (!h2.length)
				return;

			var klass = '';
			if (valid)
				if (h2.attr('id') !== active)
					$(elem).css('display', 'none');
				else
					klass = 'active';

			tabs += '<a class="' + klass + '" href="#tab-' + h2.attr('id') + '">' + h2.text() + '</a>';
		});

		nav.html(tabs);
		nav.on('click', 'a', function() {
			nav.find('a').removeClass('active');
			$(this).addClass('active');
		});

		$(window).on('hashchange', function() {
			var tab = $('#' + window.location.hash.substr(5)).parent()
			if (!tab.length)
				return;
			$('.page > div').css('display', 'none');
			tab.css('display', 'block');
		});
	};

	// Show detail for a browser (version breakdown)
	var browser_detail = function() {
		$('.chart-hbar').on('click', 'a', function(e) {
			e.preventDefault();

			var bar = $(this).closest('.chart-hbar')
			// Already open.
			if (bar.attr('data-save')) {
				bar.html(bar.attr('data-save'));
				bar.attr('data-save', '');
				return;
			}

			var name = $(this).find('small').text();
			if (!name || name === '(other)' || name === '(unknown)')
				return;

			var url = bar.attr('data-detail');
			if (!url)
				return;

			bar.attr('data-save', bar.html());
			jQuery.ajax({
				url: url,
				data: append_period({
					name:  name,
					total: bar.attr('data-total'),
				}),
				success: function(data) { bar.html(data.html); },
			});
		});
	};

	// Paginate the main path overview.
	var paginate_paths = function() {
		$('.pages-list .load-more').on('click', function(e) {
			e.preventDefault();

			jQuery.ajax({
				url: $(this).attr('data-href'),
				success: function(data) {
					$('.pages-list .count-list-pages > tbody').append(data.rows);

					if (!data.more)
						$('.pages-list .load-more').remove()
					else {
						var b = $('.pages-list .load-more');
						b.attr('data-href', b.attr('data-href') + ',' +  data.paths.join(','));
					}

					var td = $('.pages-list .total-display');
					td.text(parseInt(td.text().replace(/\s/, ''), 10) + data.total_display);
				},
			});
		});
	};

	// Paginate the referrers.
	var paginate_refs = function() {
		$('.pages-list').on('click', '.load-more-refs', function(e) {
			e.preventDefault();

			var btn = $(this);
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
				},
			});
		});
	};

	// Fill in start/end periods from buttons.
	var period_select = function() {
		$('.period-select').on('click', 'button', function(e) {
			e.preventDefault();

			var start = new Date();
			switch (this.value) {
				case 'day':       /* Do nothing */ break;
				case 'week':      start.setDate(start.getDate() - 7); break;
				case 'month':     start.setMonth(start.getMonth() - 1); break;
				case 'quarter':   start.setMonth(start.getMonth() - 3); break;
				case 'half-year': start.setMonth(start.getMonth() - 6); break;
				case 'year':      start.setFullYear(start.getFullYear() - 1); break;
				case 'all':
					start.setYear(1970);
					start.setMonth(0);
					start.setDate(1);
					break;
			}

			$('#hl-period').val(this.value);
			set_period(start, new Date())
		});

		$('.period-move').on('click', 'button', function(e) {
			e.preventDefault();
			var start = get_date($('#period-start').val()),
				end = get_date($('#period-end').val());

			switch (this.value) {
				case 'week':    start.setDate(start.getDate() - 7);   end.setDate(end.getDate() - 7);   break;
				case 'month':   start.setMonth(start.getMonth() - 1); end.setMonth(end.getMonth() - 1); break;
				case 'quarter': start.setMonth(start.getMonth() - 3); end.setMonth(end.getMonth() - 3); break;
			}

			set_period(start, end);
		});
	};

	// Select a period by dragging the mouse over a timeframe.
	var drag_timeframe = function() {
		var box, startX;

		var setpos = function(e) {
			// +1 on the right to make sure the tooltip is always visible.
			box.css(e.pageX > startX
				? {left: startX,  right: $(window).width() - e.pageX + 1}
				: {left: e.pageX, right: $(window).width() - startX + 1});
		};

		$('.chart').on('mousedown', function(e) {
			if (e.button !== 0)  // Left mouse button.
				return;

			startX = e.pageX
			box = $('<span id="drag-box"></span>').css({
				left:   e.pageX,
				right:  $(document.body).width() - e.pageX,
				top:    $(this).offset().top,
				height: $(this).outerHeight(),
			}).on('mousemove', function(e) {
				e.preventDefault();
				setpos(e);
			});

			// Mainly for Firefox.
			$(document).on('dragstart.timeframe, selectstart.timeframe', function(e) {
				e.preventDefault();
			});

			$(document.body).append(box);
		});

		$('.chart').on('mousemove', function(e) {
			e.preventDefault();
			if (!box)
				return;
			setpos(e);
		});

		$(document.body).on('mouseup', function(e) {
			if (!box)
				return;

			e.preventDefault();

			var box_left   = parseFloat(box.css('left')),
				box_right = $(window).width() - parseFloat(box.css('right')),
				start, end;
			// All charts have the same bars, so just using the first is fine.
			$('.chart').first().find('>div').each(function(i, elem) {
				var l = $(elem).offset().left,
					w = $(elem).width();

				if (!start && l + w >= box_left)
					start = elem;

				if (start && !end && l+w >= box_right) {
					end = elem;
					return false
				}
			});

			box.remove();
			box = null;
			$(document).off('.timeframe');

			set_period((start.title || start.dataset.title).split(' ')[0],
				(end.title || end.dataset.title).split(' ')[0]);
		});
	};

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list-pages').on('click', '.rlink', function(e) {
			e.preventDefault();

			var params = get_params(),
				link   = this,
				row    = $(this).closest('tr'),
				path   = row.attr('id'),
				close  = function() {
					var t = $(document.getElementById(params['showrefs']));
					t.removeClass('target');
					t.closest('tr').find('.refs').html('');
				};

			// Clicked on row that's already open, so close and stop. Don't
			// close anything yet if we're going to load another path, since
			// that gives a somewhat yanky effect (close, wait on xhr, open).
			if (params['showrefs'] === path) {
				close();
				return set_param('showrefs', null);
			}

			set_param('showrefs', path);
			jQuery.ajax({
				url: '/refs' + link.search,
				success: function(data) {
					row.addClass('target');

					if (params['showrefs'])
						close();
					row.find('.refs').html(data.rows);
					if (data.more)
						row.find('.refs').append('<a href="#_", class="load-more-refs">load more</a>')
				},
			});
		})
	};

	// Display popup when hovering a chart.
	var chart_hover = function() {
		$(document.body).on('mouseleave', '.chart', function() {
			$('#popup').remove();
		});

		// Pages chart.
		$(document.body).on('mouseenter', '.chart > div', function(e) {
			var t = $(e.target);
			if (e.target.style.length > 0)  // Inner bar (the coloured part).
				t = t.parent();

			var title = t.attr('title') || t.attr('data-title');
			if (!title)
				return;

			if (t.attr('data-title'))
				title = t.attr('data-title');
			else {
				// Reformat date and time according to site settings.
				//
				// 2019-07-22 22:00 – 22:59, 5 views
				// 2019-07-24 7:00 – 7:59, 4 views
				var split = title.split(' ');
				var date  = split[0],
					start = split[1],
					end   = split[3].replace(',', ''),
					views = ', ' + split[4] + ' ' + split[5];

				if (!SETTINGS.twenty_four_hours) {
					start = un24(start);
					end = un24(end);
				}

				if (SETTINGS.date_format !== '2006-01-02') {
					var d = new Date(),
						ds = date.split('-');
					d.setFullYear(ds[0]);
					d.setMonth(parseInt(ds[1], 10) - 1);
					d.setDate(ds[2]);
					date = format_date(d);
				}

				title = date + ' ' + start + ' – ' + end + views;
				t.attr('data-title', title);
				t.removeAttr('title');
			}

			var x = t.offset().left
			var p = $('<div id="popup"></div>').
				html(title).
				css({
					left: (x + 8) + 'px',
					top: (t.parent().position().top) + 'px',
				});

			$('#popup').remove();
			$(document.body).append(p);

			// Move to left of cursor if there isn't enough space.
			if (p.height() > 30)
				p.css('left', 0).css('left', (x - p.width() - 8) + 'px');
		});
	};

	// Get all query parameters as an object.
	var get_params = function() {
		var s = window.location.search;
		if (s.length === 0)
			return {};
		if (s[0] === '?')
			s = s.substr(1);

		var split = s.split('&'),
			obj = {};
		for (var i = 0; i < split.length; i++) {
			var item = split[i].split('=');
			obj[item[0]] = decodeURIComponent(item[1]);
		}
		return obj;
	};

	// Set query parameters to the provided object.
	var set_params = function(obj) {
		var s = [];
		for (var k in obj)
			s.push(k + '=' + encodeURIComponent(obj[k]));
		history.pushState(null, '', s.length === 0 ? '/' : ('?' + s.join('&')));
	};

	// Set one query parameter, leaving the others alone.
	var set_param = function(k, v) {
		var params = get_params();
		if (v === null)
			delete params[k];
		else
			params[k] = v;
		set_params(params);
	};

	// Convert "23:45" to "11:45 pm".
	var un24 = function(t) {
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
		if (typeof date === 'string')
			return date;
		var m = date.getMonth() + 1,
			d = date.getDate();
		return date.getFullYear() + '-' +
			(m >= 10 ? m : ('0' + m)) + '-' +
			(d >= 10 ? d : ('0' + d));
	};

	// Create Date() object from year-month-day string.
	var get_date = function(str) {
		var d = new Date(),
			s = str.split('-');
		d.setFullYear(s[0]);
		d.setMonth(parseInt(s[1], 10) - 1);
		d.setDate(s[2]);
		return d
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
		$('#period-start').val(format_date_ymd(start));
		$('#period-end').val(format_date_ymd(end));
		$('#period-form').trigger('submit');
	};
})();

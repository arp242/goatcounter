// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

(function() {
	'use strict';

	var init = function() {
		// Load settings.
		window.settings = JSON.parse(document.getElementById('settings').innerHTML);

		// Global ajax error handler.
		$(document).ajaxError(function(e, xhr, settings, err) {
			var msg = 'Could not load ' + settings.url + ': ' + err;
			console.error(msg);
			alert(msg);
		});

		period_select();
		drag_timeframe();
		load_refs();
		chart_hover();
		paginate_paths();
		paginate_refs();
		browser_detail();
	};

	// Show detail for a browser (version breakdown)
	var browser_detail = function() {
		$('.browsers-list').on('click', 'a', function(e) {
			e.preventDefault();

			var bar = $(this).closest('.chart')
			if (bar.attr('data-save')) {
				bar.html(bar.attr('data-save'));
				bar.attr('data-save', '');
				return;
			}

			var browser = $(this).attr('data-browser');
			if (!browser)
				return;

			bar.attr('data-save', bar.html());
			jQuery.ajax({
				url: '/browsers',
				data: {
					'period-start': $('#period-start').val(),
					'period-end':   $('#period-end').val(),
					'browser':      browser,
					'total':        bar.attr('data-total'),
				},
				dataType: 'json',
				success: function(data) {
					bar.html(data.html);
				},
			});

		});
	};

	// Paginate the main path overview.
	var paginate_paths = function() {
		$('.pages-list .load-more').on('click', function(e) {
			e.preventDefault();

			jQuery.ajax({
				url: $(this).attr('data-href'),
				dataType: 'json',
				success: function(data) {
					$('.pages-list .count-list-pages > tbody').append(data.rows);

					var b = $('.pages-list .load-more');
					b.attr('data-href', b.attr('data-href') + ',' +  data.paths.join(','));

					var td = $('.pages-list .total-display');
					td.text(parseInt(td.text().replace(/\s/, ''), 10) + data.total_display);

					if (!data.more)
						$('.pages-list .load-more').remove()
				},
			});
		});
	};

	// Paginate the referrers.
	var paginate_refs = function() {
		// TODO: won't work w/o JS.
		$('.pages-list').on('click', '.load-more-refs', function(e) {
			e.preventDefault();

			var btn = $(this);
			jQuery.ajax({
				url: '/refs',
				data: {
					'showrefs':     btn.closest('tr').attr('id'),
					'period-start': $('#period-start').val(),
					'period-end':   $('#period-end').val(),
					'offset':       btn.prev().find('tr').length,
				},
				dataType: 'json',
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
				case 'day':       /* Do nothing */; break;
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

			$('#period-start').val(format_date_ymd(start));
			$('#period-end').val(format_date_ymd(new Date()));
			$('#hl-period').val(this.value);
			$(this).closest('form').trigger('submit');
		});
	};

	// Select a period by dragging the mouse over a timeframe.
	var drag_timeframe = function() {
		return;

		/*
		var down, box;
		$('.chart').on('mousedown', function(e) {
			down = e;
			box = $('<span id="drag-box"></span>').css({
				left: e.pageX,
				top: e.pageY,
			});
			$(this).append(box);
		});

		$('.chart').on('mousemove', function(e) {
			e.preventDefault();

			if (!down)
				return;

			box.css({
				width: e.pageX,
				height: e.pageY,
			});
		});

		$('.chart').on('mouseup', function(e) {
			if (!down)
				return;

			down = null;
		});
		*/
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

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list-pages').on('click', '.rlink', function(e) {
			e.preventDefault();

			var params = get_params(),
				link = this,
				row = $(this).closest('tr'),
				path = row.attr('id');


			var close = function() {
				var t = $(document.getElementById(params['showrefs']));
				t.removeClass('target');
				t.closest('tr').find('.refs').html('');
			};

			// Clicked on row that's already open, so just close and stop. Don't
			// close anything yet if we're going to load another path, since
			// that gives a somewhat yanky effect (close, wait on xhr, open).
			if (params['showrefs'] === path) {
				close();
				return set_param('showrefs', null);
			}

			set_param('showrefs', path);
			jQuery.ajax({
				url: '/refs' + link.search,
				dataType: 'json',
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
		$(document.body).on('mouseleave', '.chart', function(e) {
			$('#popup').remove();
		});

		$(document.body).on('mouseenter', '.chart > *', function(e) {
			var hbar = e.target.tagName.toLowerCase() === 'div';
			if (hbar && e.target.style.length > 0)
				var t = $(e.target.parentNode);
			else
				var t = $(e.target);

			var title = t.attr('title');
			if (!title) {
				title = t.attr('data-title');
				if (!title)
					return;
			}
			else if (hbar) {
				// Reformat date and time according to site settings. This won't
				// work for non-JS users, but doing this on the template site
				// would make caching harder. It's a fair compromise.
				//
				// 2019-07-22 22:00 – 22:59, 5 views
				// 2019-07-24 7:00 – 7:59, 4 views
				var split = title.split(' ');
				var date  = split[0],
					start = split[1],
					end   = split[3].replace(',', ''),
					views = ', ' + split[4] + ' ' + split[5];

				if (!window.settings.twenty_four_hours) {
					start = un24(start);
					end = un24(end);
				}

				if (window.settings.date_format !== '2006-01-02') {
					var d = new Date(),
						ds = date.split('-');
					d.setFullYear(ds[0]);
					d.setMonth(parseInt(ds[1], 10) - 1);
					d.setDate(ds[2]);
					date = format_date(d);
				}

				title = date + ' ' + start + ' – ' + end + views;
			}

			var x = t.offset().left
			var p = $('<div id="popup"></div>').
				html(title).
				css({
					left: (x + 8) + 'px',
					top: (t.parent().position().top) + 'px',
				});

			t.attr('data-title', title);
			t.removeAttr('title');

			$('#popup').remove();
			$(document.body).append(p);

			// Move to left of cursor if there isn't enough space.
			if (p.height() > 30)
				p.css('left', 0).css('left', (x - p.width() - 8) + 'px');
		});
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
			items = window.settings.date_format.split(/[-/\s]/),
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
		if (window.settings.date_format.indexOf('/') > -1)
			joiner = '/';
		else if (window.settings.date_format.indexOf(' ') > -1)
			joiner = ' ';
		return new_date.join(joiner);
	};

	// Format a date at year-month-day.
	var format_date_ymd = function(date) {
		var m = date.getMonth() + 1,
			d = date.getDate();
		return date.getFullYear() + '-' +
			(m >= 10 ? m : ('0' + m)) + '-' +
			(d >= 10 ? d : ('0' + d));
	}

	$(document).ready(init);
})();

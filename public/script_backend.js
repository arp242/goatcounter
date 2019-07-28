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
	};

	// Fill in start/end periods from buttons.
	var period_select = function() {
		$('.period-select').on('click', 'button', function(e) {
			e.preventDefault();

			// TODO(v1): also set on load.
			//$('.period-select button').removeClass('active');
			//$(this).addClass('active');

			var start = new Date();
			switch (this.value) {
				case 'day':       start.setDate(start.getDate() - 1); break;
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

			$(this).closest('form').trigger('submit');
		})
	};

	// Select a period by dragging the mouse over a timeframe.
	var drag_timeframe = function() {
		// TODO(v1): finish.
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

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list').on('click', '.rlink', function(e) {
			e.preventDefault();

			var hash = decodeURIComponent(location.hash.substr(1)),
				link = this,
				row = $(this).closest('tr');

			// Close existing.
			if (hash !== '') {
				var t = $(document.getElementById(hash));
				t.removeClass('target');
				t.closest('tr').find('.refs').html('');
 
				if (hash === row.attr('id')) {
					var nl = link.href.substr(0, link.href.indexOf('#'));
					nl = nl.replace(/showrefs=.*?&/, '&'); // TODO(v1): do better!
					history.pushState(null, "", nl);

					location.hash = '_'; // '_' and not '' so we won't scroll to top.
					return;
				}
			}

			jQuery.ajax({
				url: '/refs' + this.search,
				dataType: 'text',
				success: function(data) {
					row.find('.refs').html(data);

					// TODO(v1): make back button work by hooking in to hashchange
					// or something.
					history.pushState(null, "", link.href);
					row.addClass('target');
				},
			});
		})
	};

	// Display popup when hovering a chart.
	var chart_hover = function() {
		$('.chart').on('mouseleave', function(e) {
			$('#popup').remove();
		});

		$('.chart').on('mouseenter', '> div', function(e) {
			if (e.target.style.length > 0)
				var t = $(e.target.parentNode);
			else
				var t = $(e.target);

			// Reformat date and time according to site settings. This won't
			// work for non-JS users, but doing this on the template site would
			// make caching harder. It's a fair compromise.
			var title = t.attr('title');
			if (!title)
				title = t.attr('data-title')
			else {
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

			var p = $('<div id="popup"></div>').
				html(title).
				css({
					left: (e.pageX + 8) + 'px',
					top: (t.parent().position().top) + 'px',
				});

			t.attr('data-title', title);
			t.removeAttr('title');

			$('#popup').remove();
			$(document.body).append(p);

			// Move to left of cursor if there isn't enough space.
			if (p.height() > 30)
				p.css('left', 0).css('left', (e.pageX - p.width() - 8) + 'px');
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

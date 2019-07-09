(function() {
	'use strict';

	var init = function() {
		period_select();
		drag_timeframe();
		load_refs();
		chart_hover();
	};

	// Fill in start/end periods from buttons.
	var period_select = function() {
		$('.period-select').on('click', 'button', function(e) {
			e.preventDefault();

			// TODO: also set on load.
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

			$('#period-start').val(format_date(start));
			$('#period-end').val(format_date(new Date()));
		})

		// TODO: still selects text in Firefox...
		$('.period-select').on('dblclick', 'button', function(e) {
			e.preventDefault();
			$(this).closest('form').trigger('submit');
		});
	};

	// Select a period by dragging the mouse over a timeframe.
	var drag_timeframe = function() {
		// TODO: finish.
		return;

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
	};

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list').on('click', '.rlink', function(e) {
			e.preventDefault();

			var hash = decodeURIComponent(location.hash.substr(1)),
				link = this,
				row = $(this).closest('tr');
			if (hash !== '') {
				$(document.getElementById(hash)).closest('tr').find('.refs').html('');

				console.log(hash, row.attr('id'));
				if (hash === row.attr('id'))
					return;
			}

			jQuery.ajax({
				url: '/refs' + this.search,
				dataType: 'text',
				success: function(data) {
					row.find('.refs').html(data);

					// TODO: set target?
					// TODO: make back button work!
					history.pushState(null, "", link.href);
					location.hash = link.hash;
				},
				// TODO: global error handler?
				// error: ( jqXHR jqXHR, String textStatus, String errorThrown ) {
				// window.location = ...
				// }
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

			var p = $('<div id="popup"></div>').
				html(t.attr('title') || t.attr('data-title')).
				css({
					// TODO: move to left of cursor if there isn't enough space.
					left: (e.pageX + 8) + 'px',
					top: (t.parent().position().top) + 'px',
				});

			t.attr('data-title', t.attr('title'));
			t.removeAttr('title');

			$('#popup').remove();
			$(document.body).append(p);
		});
	};

	// Format a date as year-month-day
	// TODO: user config!
	var format_date = function(date) {
		var m = date.getMonth() + 1;
		var d = date.getDate();

		return date.getFullYear() + '-' +
			(m >= 10 ? m : ('0' + m)) + '-' +
			(d >= 10 ? d : ('0' + d));
	};

	$(document).ready(init);
})();

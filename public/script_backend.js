(function() {
	'use strict';

	var init = function() {
		period_select();
		load_refs();
	};

	// Fill in start/end periods from buttons.
	var period_select = function() {
		$('.period-select').on('click', 'button', function(e) {
			e.preventDefault();

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

	// Format a date as year-month-day
	// TODO: user config!
	var format_date = function(date) {
		var m = date.getMonth() + 1;
		var d = date.getDate();

		return date.getFullYear() + '-' +
			(m >= 10 ? m : ('0' + m)) + '-' +
			(d >= 10 ? d : ('0' + m));
	};

	$(document).ready(init);
})();

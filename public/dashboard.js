// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

;(function() {
	'use strict';

	// Set up the entire dashboard page.
	var page_dashboard = function() {
		;[dashboard_widgets, hdr_select_period, hdr_datepicker, hdr_filter, hdr_views, hdr_sites, translate_locations].forEach((f) => f.call())
	}
	window.page_dashboard = page_dashboard  // Directly setting window loses the name attr 🤷

	// Set up all the dashboard widget contents (but not the header).
	var dashboard_widgets = function() {
		;[draw_chart, paginate_pages, load_refs, hchart_detail, ref_pages, bind_scale].forEach((f) => f.call())
	}

	// Get the Y-axis scale.
	var get_original_scale = function(current) { return $('.count-list-pages').attr('data-max') }
	var get_current_scale  = function(current) { return $('.count-list-pages').attr('data-scale') }

	// Reload the widgets on the dashboard.
	var reload_dashboard = function(done) {
		jQuery.ajax({
			url:     '/',
			data:    append_period({
				daily:     $('#daily').is(':checked'),
				'as-text': $('#as-text').is(':checked'),
				max:       get_original_scale(),
				reload:    't',
			}),
			success: function(data) {
				$('#dash-widgets').html(data.widgets)
				$('#dash-timerange').html(data.timerange)
				dashboard_widgets()
				highlight_filter($('#filter-paths').val())
				if (done)
					done()
			},
		})
	}

	// Append period-start and period-end values to the data object.
	var append_period = function(data) {
		data = data || {}
		data['period-start'] = $('#period-start').val()
		data['period-end']   = $('#period-end').val()
		data['filter']       = $('#filter-paths').val()
		return data
	}

	// Set the start and end period and submit the form.
	var set_period = function(start, end) {
		if (TZ_OFFSET) {
			var offset = (start.getTimezoneOffset() + TZ_OFFSET) / 60
			start.setHours(start.getHours() + offset)
			end.setHours(end.getHours() + offset)
		}

		$('#period-start').val(format_date_ymd(start))
		$('#period-end').val(format_date_ymd(end))
		$('#dash-form').trigger('submit')
	}

	// Highlight a filter pattern in the path and title.
	var highlight_filter = function(s) {
		if (s === '')
			return
		$('.pages-list .count-list-pages > tbody.pages').find('.rlink, .page-title:not(.no-title)').each(function(_, elem) {
			if ($(elem).find('b').length)  // Don't apply twice after pagination
				return
			elem.innerHTML = elem.innerHTML.replace(new RegExp('' + quote_re(s) + '', 'gi'), '<b>$&</b>')
		})
	}

	// Fill in start/end periods from buttons.
	var hdr_select_period = function() {
		// Reload dashboard when clicking a checkbox.
		$('#dash-main input[type="checkbox"]').on('click', function(e) {
			$('#hl-period').attr('disabled', false)
			$('#dash-form').trigger('submit')
		})

		$('#dash-select-period').on('click', 'button', function(e) {
			e.preventDefault()

			var start = new Date(), end = new Date()
			switch (this.value) {
				case 'day':       /* Do nothing */ break
				case 'week':      start.setDate(start.getDate() - 7);   break;
				case 'month':     start.setMonth(start.getMonth() - 1); break;
				case 'quarter':   start.setMonth(start.getMonth() - 3); break;
				case 'half-year': start.setMonth(start.getMonth() - 6); break;
				case 'year':      start.setFullYear(start.getFullYear() - 1); break;
				case 'week-cur':
					if (USER_SETTINGS.sunday_starts_week)
						start.setDate(start.getDate() - start.getDay())
					else
						start.setDate(start.getDate() - start.getDay() + (start.getDay() ? 1 : -6))
					end = new Date(start.getFullYear(), start.getMonth(), start.getDate() + 6)
					break;
				case 'month-cur':
					start.setDate(1)
					end = new Date(end.getFullYear(), end.getMonth() + 1, 0)
					break
			}

			$('#hl-period').val(this.value).attr('disabled', false)
			set_period(start, end)
		})

		$('#dash-move').on('click', 'button', function(e) {
			e.preventDefault()
			var start = get_date($('#period-start').val()),
			    end   = get_date($('#period-end').val())

			// TODO: make something nicer than alert()s.
			if (this.value.substr(-2) === '-f' && end.getTime() > (new Date()).getTime())
				return alert(T('error/date-future'))

			switch (this.value) {
				case 'day-b':     start.setDate(start.getDate()   - 1); end.setDate(end.getDate()   - 1); break;
				case 'week-b':    start.setDate(start.getDate()   - 7); end.setDate(end.getDate()   - 7); break;
				case 'month-b':   start.setMonth(start.getMonth() - 1); end.setMonth(end.getMonth() - 1); break;
				case 'day-f':     start.setDate(start.getDate()   + 1); end.setDate(end.getDate()   + 1); break;
				case 'week-f':    start.setDate(start.getDate()   + 7); end.setDate(end.getDate()   + 7); break;
				case 'month-f':   start.setMonth(start.getMonth() + 1); end.setMonth(end.getMonth() + 1); break;
			}
			if (start.getDate() === 1 && this.value.substr(0, 5) === 'month')
				end = new Date(start.getFullYear(), start.getMonth() + 1, 0)

			if (start > (new Date()).getTime())
				return alert(T('error/date-future'))
			if (SITE_FIRST_HIT_AT > end.getTime())
				return alert(T('error/date-past'))

			$('#dash-select-period').attr('class', '')
			set_period(start, end);
		})
	}

	// Setup datepicker fields.
	var hdr_datepicker = function() {
		$('#dash-form').on('submit', function(e) {
			// Remove the "off" checkbox placeholders.
			$('#dash-form :checked').each((_, c) => $(`input[name="${c.name}"][value="off"]`).prop('disabled', true))

			if (get_date($('#period-start').val()) <= get_date($('#period-end').val()))
				return

			e.preventDefault()
			if (!$('#period-end').hasClass('red'))
				$('#period-end').addClass('red').after(' <span class="red">' + T('error/date-mismatch') + '</span>')
		})

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

		var opts = {
			toString: format_date_ymd,
			parse:    get_date,
			firstDay: USER_SETTINGS.sunday_starts_week ? 0 : 1,
			minDate:  new Date(SITE_FIRST_HIT_AT),
			i18n: {
				ariaLabel:     T('datepicker/keyboard'),
				previousMonth: T('datepicker/month-prev'),
				nextMonth:     T('datepicker/month-next'),
				weekdays:      days,
				weekdaysShort: daysShort,
				months:        months,
			},
		}
		new Pikaday($('#period-start')[0], opts)
		new Pikaday($('#period-end')[0], opts)
	}

	// Reload the dashboard when typing in the filter input, so the user won't
	// have to press "enter".
	var hdr_filter = function() {
		highlight_filter($('#filter-paths').val())

		$('#filter-paths').on('keydown', function(e) {
			if (e.keyCode === 13)  // Don't submit form on enter.
				e.preventDefault()
		})

		var t
		$('#filter-paths').on('input', function(e) {
			clearTimeout(t)
			t = setTimeout(function() {
				var filter = $(e.target).val().trim()
				push_query({filter: filter, showrefs: null})
				$('#filter-paths').toggleClass('value', filter !== '')

				var loading = $('<span class="loading"></span>')
				$(e.target).after(loading)
				// TODO: back button doesn't quite work with this.
				reload_dashboard(() => loading.remove())
			}, 300)
		})
	}

	// Save current view.
	var hdr_views = function() {
		$('#dash-saved-views >span').on('click', function(e) {
			e.preventDefault()

			var d = $('#dash-saved-views >div')
			d.css('display', d.css('display') === 'block' ? 'none' : 'block')

			var close = () => { d.css('display', 'none'); $('body').off('.saved-views') }
			$('body').on('keydown.saved-views', (e) => { if (e.keyCode === 27) close() })
			$('body').on('click.saved-views',   (e) => { if (!$(e.target).closest('#dash-saved-views').length) close() })
		})

		$('.save-current-view').on('click', function(e) {
			e.preventDefault()
			var p = $('#dash-select-period').attr('class').substr(7)
			if (p === '')
				p = (get_date($('#period-end').val()) - get_date($('#period-start').val())) / 86400000

			var done = paginate_button($(this), () => {
				jQuery.ajax({
					url:    '/user/view',
					method: 'POST',
					data: {
						csrf:      CSRF,
						name:      'default',
						filter:    $('#filter-paths').val(),
						daily:     $('#daily').is(':checked'),
						'as-text': $('#as-text').is(':checked'),
						period:    p,
					},
					success: () => {
						done()
						var s = $('<em> ' + T('notify/saved') + '</em>')
						$(this).after(s)
						setTimeout(() => s.remove(), 2000)
					},
				})
			})
		})
	}

	// Set up the "site switcher".
	var hdr_sites = function() {
		var list       = $('.sites-list'),
			select     = $('.sites-list-select'),
			list_width = list.width()

		select.on('change', function() { window.location = this.value })

		// The sites-list has 'visibility: hidden' on initial load, so we can
		// get the rendered width; only need to do this once as it won't change.
		$(window).on('resize', function() {
			var show_dropdown = list_width > $('nav.center').width() - $('#usermenu').width() -
			                    $('.sites-header').width() - (15 * window.devicePixelRatio)

			select.css('display', show_dropdown ? 'inline-block' : 'none')
			if (show_dropdown)
				list.css('display', 'none')
			else
				list.css({display: 'inline', visibility: 'visible'})
		}).trigger('resize')

		// Load as absolute initially, so an overflowing list won't cause the
		// page to wobble on load.
		list.css('position', 'static')
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

	// Bind the Y-axis scale actions.
	var bind_scale = function() {
		$('.count-list').on('click', '.rescale', function(e) {
			e.preventDefault()

			var scale = $(this).closest('.chart').attr('data-max')
			$('.pages-list .scale').html(format_int(scale))
			$('.pages-list .count-list-pages').attr('data-scale', scale)
			$('.pages-list .chart-bar').each((_, c) => { c.dataset.done = '' })
			draw_chart()
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

			var is_pages = $(chart).closest('.count-list-pages').length > 0
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
					if (is_pages && scale && scale !== 1) {
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

	// Translate country names; we do this in JavaScript with Intl, which works
	// fairly well and keeps the backend/database a lot simpler.
	var translate_locations = function() {
		if (!window.Intl || !window.Intl.DisplayNames)
			return;

		var names = new Intl.DisplayNames([USER_SETTINGS.language], {type: 'region'})
		var set = function(chart) {
			chart.find('div[data-key]').each((_, e) => {
				if (e.dataset.key.substr(0, 1) === '(') // Skip "(unknown)"
					return
				var n = names.of(e.dataset.key)
				if (n)
					$(e).find('.col-name .bar-c .cutoff').text(n)
			})
		}

		USER_SETTINGS.widgets.forEach((w, i) => {
			if (w.n === 'locations') {
				var chart = $(`.hchart[data-widget=${i}]`)
				set(chart)

				var obs = new MutationObserver((mut) => {
					if (mut[0].addedNodes.length === 0 || mut[0].addedNodes[0].className !== 'rows')
						return
					obs.disconnect()  // Not strictly needed, but just in case to prevent infinite looping.
					set(chart)
					obs.observe(chart.find('.rows')[0], {childList: true})
				})
				obs.observe(chart.find('.rows')[0], {childList: true})
			}
		})
	}

	// Paginate the main path overview.
	var paginate_pages = function() {
		$('.pages-list >.load-more').on('click', function(e) {
			e.preventDefault()

			var btn   = $(this),
				pages = $(this).closest('.pages-list')
			var done = paginate_button(btn, () => {
				jQuery.ajax({
					url:  '/load-widget',
					data: append_period({
						widget:    pages.attr('data-widget'),
						daily:     $('#daily').is(':checked'),
						exclude:   pages.find('.count-list-pages >tbody >tr').toArray().map((e) => e.dataset.id).join(','),
						max:       get_original_scale(),
						'as-text': $('#as-text').is(':checked'),
					}),
					success: function(data) {
						pages.find('.count-list-pages >tbody.pages').append(data.html)
						draw_chart()

						highlight_filter($('#filter-paths').val())
						btn.css('display', data.more ? 'inline-block' : 'none')

						pages.find('.total-unique-display').each((_, t) => {
							$(t).text(format_int(parseInt($(t).text().replace(/[^0-9]/, ''), 10) + data.total_unique_display))
						})

						done()
					},
				})
			})
		})
	}

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list-pages').on('click', '.load-refs, .hchart .load-more', function(e) {
			e.preventDefault()

			var params = split_query(location.search),
				btn    = $(this),
				row    = btn.closest('tr'),
				widget = row.closest('.pages-list').attr('data-widget'),
				path   = row.attr('id'),
				init   = btn .is('.load-refs'),
				close  = function() {
					var t = $(document.getElementById(params['showrefs']))
					t.removeClass('target')
					t.closest('tr').find('.refs').html('')
				}

			// Clicked on row that's already open, so close and stop. Don't
			// close anything yet if we're going to load another path, since
			// that gives a somewhat yanky effect (close, wait on xhr, open).
			if (init && params['showrefs'] === path) {
				close()
				return push_query({showrefs: null})
			}

			push_query({showrefs: path})
			var done = paginate_button(btn , () => {
				jQuery.ajax({
					url:   '/load-widget',
					data: append_period({
						widget: widget,
						key:    path,
						total:  row.find('>.col-count').text().replace(/[^0-9]+/g, ''),
						offset: row.find('.refs .rows>div').length,
					}),
					success: function(data) {
						row.addClass('target')

						if (init) {
							if (params['showrefs'])
								close()
							row.find('.refs').html(data.html)
						}
						else {
							row.find('.refs .rows').append($(data.html).find('>div'))
							if (!data.more)
								btn.css('display', 'none')
						}
						done()
					},
				})
			})
		})
	}

	// Paginate and show details for the horizontal charts.
	var hchart_detail = function() {
		var get_total = () => $('.js-total-unique-utc').text()

		// Paginate the horizontal charts.
		$('.hcharts').on('click', '.load-more', function(e) {
			e.preventDefault();

			var btn   = $(this),
				chart = btn.closest('.hchart'),
				key   = chart.attr('data-key'),
				rows  = chart.find('>.rows')
			var done = paginate_button($(this), () => {
				jQuery.ajax({
					url:  '/load-widget',
					data: append_period({
						widget: chart.attr('data-widget'),
						total:  get_total(),
						key:    key,
						offset: rows.find('div:not(.hchart)').length,
					}),
					success: function(data) {
						rows.append($(data.html).find('>div'))
						if (!data.more)
							btn.css('display', 'none')
						done()
					},
				})
			})
		})

		// Load detail.
		$('.hchart').on('click', '.load-detail', function(e) {
			e.preventDefault()

			var btn    = $(this),
				row    = btn.closest('div[data-key]'),
				chart  = row.closest('.hchart'),
				widget = chart.attr('data-widget'),
				key    = row.attr('data-key')
			if (row.next().is('.detail'))
				return row.next().remove()

			var l = btn.find('.bar-c')
			l.addClass('loading')
			var done = paginate_button(l, () => {
				jQuery.ajax({
					url:     '/load-widget',
					data:    append_period({
						widget: widget,
						key:    key,
						total:  get_total(),
						//offset: rows.find('>div').length,
					}),
					success: function(data) {
						chart.find('.detail').remove()
						row.after($(`<div class="hchart detail" data-widget="${widget}" data-key="${key}"></div>`).html(data.html))
						done()
					},
				})
			})
		})
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
	}

	// Join query parameters from {k: v} object to href.
	var join_query = function(obj) {
		var s = [];
		for (var k in obj)
			s.push(k + '=' + encodeURIComponent(obj[k]));
		return (s.length === 0 ? '/' : ('?' + s.join('&')));
	}

	// Set one query parameter – leaving the others alone – and push to history.
	var push_query = function(params) {
		var current = split_query(location.search)
		for (var k in params) {
			if (params[k] === null)
				delete current[k]
			else
				current[k] = params[k]
		}
		history.pushState(null, '', join_query(current))
	}

	// Check if this is a mobile browser. Probably not 100% reliable.
	var is_mobile = () => navigator.userAgent.match(/Mobile/i) || (window.innerWidth <= 800 && window.innerHeight <= 600)

	// Quote special regexp characters. https://locutus.io/php/pcre/preg_quote/
	var quote_re = (s) => s.replace(new RegExp('[.\\\\+*?\\[\\^\\]$(){}=!<>|:\\-]', 'g'), '\\$&')
})();

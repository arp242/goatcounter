// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

;(function() {
	'use strict';

	// Set up the entire dashboard page.
	var page_dashboard = function() {
		;[dashboard_widgets, hdr_select_period, hdr_datepicker, hdr_filter, hdr_views, hdr_sites,
			translate_locations, dashboard_loader, configure_widgets,
		].forEach((f) => f.call())
	}
	window.page_dashboard = page_dashboard  // Directly setting window loses the name attr 🤷

	// Set up all the dashboard widget contents (but not the header).
	var dashboard_widgets = function() {
		;[init_charts, paginate_pages, load_refs, hchart_detail, ref_pages, bind_scale].forEach((f) => f.call())
	}

	// Open websocket for the dashboard loader.
	var dashboard_loader = function() {
		if (!window.USE_WEBSOCKET)
			return
		if (window.WEBSOCKET && window.WEBSOCKET.readyState <= 1)
			return

		let cid  = $('#js-connect-id').text()
		window.WEBSOCKET = new WebSocket((location.protocol === 'https:' ? 'wss://' : 'ws://') + document.location.host + BASE_PATH + '/loader?id=' + cid)
		window.WEBSOCKET.onmessage = function(e) {
			let msg = JSON.parse(e.data),
				wid = $(`#dash-widgets div[data-widget=${msg.id}]`)
			wid.html(msg.html)
			draw_all_charts()

			if (wid.hasClass('pages-list'))
				dashboard_widgets()
		}
	}

	// Setup the configure widgets buttons.
	var configure_widgets = function() {
		$('#dash-widgets').on('click', '.configure-widget', function(e) {
			e.preventDefault()

			let pop,
				btn    = $(this),
				pos    = btn.offset(),
				wid    = btn.closest('[data-widget]').attr('data-widget'),
				url    = BASE_PATH + '/user/dashboard/' + wid,
				remove = function() {
					pop.remove()
					$(document.body).off('.unpop')
				},
				save = function(e) {
					e.preventDefault()
					remove()
					jQuery.ajax({
						url:  url,
						type: 'post',
						data: `${pop.serialize()}&csrf=${encodeURIComponent(CSRF)}`,
						success: function(data) {
							reload_dashboard()
							// TODO: fix reload_widget(); has some odd behaviour.
							//reload_widget(wid, {}, null)
						}
					})
				}

			jQuery.ajax({
				url:     url,
				success: function(data) {
					pop = $(data).css({left: pos.left + 'px', top: (pos.top + 10) + 'px'}).on('submit', save)
					$(document.body).append(pop).on('click.unpop', function(e) {
						if ($(e.target).closest('.widget-settings').length)
							return
						remove()
					})
				},
			})
		})
	}

	// Get the Y-axis scale.
	var get_original_scale = function() { return parseInt($('.count-list-pages').attr('data-max'), 0) }
	var get_current_scale  = function() { return parseInt($('.count-list-pages').attr('data-scale'), 0) }

	// Reload a single widget.
	var reload_widget = function(wid, data, done) {
		data = data || {}
		data['widget'] = wid
		data['daily']  = $('#daily').is(':checked')
		data['max']    = get_original_scale()
		data['total']  = $('.js-total-utc').text()

		jQuery.ajax({
			url:  BASE_PATH + '/load-widget',
			type: 'get',
			data: append_period(data),
			success: function(data) {
				if (done)
					done()
				else {
					$(`[data-widget="${wid}"]`).html(data.html)
					dashboard_widgets()
					highlight_filter($('#filter-paths').val())
				}
			},
		})
	}

	// Reload all widgets on the dashboard.
	var reload_dashboard = function(done) {
		jQuery.ajax({
			url:     BASE_PATH + '/',
			data:    append_period({
				daily:     $('#daily').is(':checked'),
				max:       get_original_scale(),
				reload:    't',
				connectID: $('#js-connect-id').text(),
			}),
			success: function(data) {
				$('#dash-widgets').html(data.widgets)
				$('#dash-timerange').html(data.timerange)
				dashboard_widgets()
				redraw_all_charts()
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
		// Not done on any desktop OS as styling these fields with basic stuff
		// (like setting a cross-browser consistent height) is really hard and
		// fraught with all sort of idiocy. They also don't really look all that
		// great and the UX is frankly bad.
		//
		// Also do this if Pikaday is undefined; this should never happen, but
		// I've seen some errors for this.
		if (is_mobile() || !window.Pikaday) {
			return $('#period-start, #period-end').
				attr('type', 'date').
				css('width', 'auto').  // Make sure there's room for UI chrome.
				on('change', () => { $('#dash-form').trigger('submit') })
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
		$('#period-start, #period-end').attr('type', 'text')
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
					url:    BASE_PATH + '/user/view',
					method: 'POST',
					data: {
						csrf:      CSRF,
						name:      'default',
						filter:    $('#filter-paths').val(),
						daily:     $('#daily').is(':checked'),
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
					url: BASE_PATH + '/pages-by-ref',
					data: append_period({name: btn.text()}),
					success: function(data) {
						p.append(data.html)
						done()
					}
				})
			})
		})
	}

	// Keep an array of charts so we can stop them on resize, otherwise the
	// resize and mouse event will be bound twice.
	//
	// TODO: this is rather ugly; charty.js should handle this really. Actually,
	// what we really want is that calling charty() will just stop/unbind any
	// previous charts; but removeEventListener will only accept a function
	// reference. Should implement some simply jQuery-like namespaces:
	//
	//   canvas.on('mousemouse.charty', ...)
	//
	// So we can then do:
	//
	//   canvas.off('.charty', ...)
	var charts = []

	// Bind the Y-axis scale actions.
	var bind_scale = function() {
		$('.count-list').on('click', '.rescale', function(e) {
			e.preventDefault()

			var scale = $(this).closest('.chart').attr('data-max')
			$('.pages-list .scale').html(format_int(scale))
			$('.pages-list .count-list-pages').attr('data-scale', scale)

			charts.forEach((c) => {
				c.ctx().canvas.dataset.done = ''
				c.stop()
			})
			charts = []
			draw_all_charts()
		})
	}

	var redraw_all_charts = function() {
		$('#tooltip').remove()
		charts.forEach((c) => {
			c.ctx().canvas.dataset.done = ''
			c.stop()
		})
		charts = []
		draw_all_charts()
	}

	var init_charts = function() {
		$(window).on('resize', redraw_all_charts)
		window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', redraw_all_charts)

		// force-dark manually added or removed.
		new MutationObserver(function(muts, observer) {
			muts.forEach((m) => m.type === 'attributes' && m.attributeName === 'class' && redraw_all_charts())
		}).observe($('html')[0], {attributes: true})

	}

	// Draw all charts.
	var draw_all_charts = function() {
		$('.chart-line, .chart-bar').each(function(i, chart) {
			// Use setTimeout to force the browser to actually render this ASAP;
			// without it, the charts will all be displayed at the same time,
			// rather than one-by-one as they're generated.
			//
			// It's not super-slow to make them, but it's just a split second
			// where the chart is blank, and it's better with this, especially
			// on long timeviews and/or with many pages displayed at once.
			//
			// TODO: possibly move this to charty.js?
			setTimeout(() => draw_chart(chart), 0)
		})
	}

	// Draw this chart
	var draw_chart = function(c) {
		let canvas = $(c).find('canvas')[0]
		if (!canvas || canvas.dataset.done === 't')
			return
		canvas.dataset.done = 't'

		let stats = JSON.parse(c.dataset.stats)
		if (!stats)
			return

		let ctx     = canvas.getContext('2d', {alpha: false}),
			max     = Math.max(10, parseInt(c.dataset.max, 10)),
			scale   = get_current_scale(),
			daily   = c.dataset.daily === 'true',
			isBar   = $(c).is('.chart-bar'),
			isEvent = $(c).closest('tr').hasClass('event'),
			isPages = $(c).closest('.count-list-pages').length > 0,
			ndays   = (get_date($('#period-end').val()) - get_date($('#period-start').val())) / (86400*1000)

		if (isPages && scale)
			max = scale

		var data
		if (daily)
			data = stats.map((s) => [s.daily]).reduce((a, b) => a.concat(b))
		else
			data = stats.map((s) => s.hourly).reduce((a, b) => a.concat(b))

		let futureFrom = 0
		var chart = charty(ctx, data, {
			mode: isBar ? 'bar' : 'line',
			max:  max,
			line: {
				color: style('chart-line'),
				fill:  style('chart-fill'),
				width: daily || ndays <= 14 ? 1.5 : 1
			},
			bar:  {color: style('chart-line')},
			done: (chart) => {
				// Show future as greyed out.
				let last   = stats[stats.length - 1].day + (daily ? '' : ' 23:59:59'),
					future = last > format_date_ymd(new Date()) + (daily ? '' : ' 23:59:59')
				if (future) {
					let dpr   = Math.max(1, window.devicePixelRatio || 1),
						width = chart.barWidth() * ((get_date(last) - new Date()) / ((daily ? 86400 : 3600) * 1000))
					futureFrom = canvas.width/dpr - width - chart.pad()

					ctx.fillStyle = '#ddd'
					ctx.beginPath()
					ctx.fillRect(futureFrom, (chart.pad()-1), width, canvas.height/dpr - chart.pad()*2 + 2)
				}
			},
		})
		charts.push(chart)

		// Show tooltip and highlight position on mouse hover.
		var tip   = $('<div id="tooltip"></div>'),
			reset = {x: -1, y: -1, f: () => {}}
		chart.mouse(function(i, x, y, w, h, offset, ev) {
			if (ev == 'leave') {
				tip.remove()
				reset.f()
				return
			}
			else if (ev === 'enter') { }
			else if (x === reset.x)
				return

			let dpr    = Math.max(1, window.devicePixelRatio || 1),
				day    = daily ? stats[i] : stats[Math.floor(i / 24)],
				start  = (i % 24) + ':00',
				end    = (i % 24) + ':59',
				visits = daily ? day.daily : day.hourly[i%24],
				views  = daily ? day.daily : day.hourly[i%24]

			let title = '',
				future = futureFrom && x >= futureFrom - 1
			if (daily)
				title = `${format_date(day.day, true)}`
			else
				title = `${format_date(day.day, true)} ${un24(start)} – ${un24(end)}`
			if (future)
				title += '; ' + T('dashboard/future')
			if (!future && !USER_SETTINGS.fewer_numbers) {
				if (isEvent) {
					title += '; ' + T('dashboard/tooltip-event', {
						unique: format_int(visits),
						clicks: `<span class="views">${format_int(views)}`,
					}) + '</span>'
				}
				else {
					title += '; ' + T('dashboard/totals/num-visits', {
						'num-visits': format_int(visits),
					}) + '</span>'
				}
			}

			tip.remove()
			tip.html(title)
			$('body').append(tip)
			tip.css({
				left: (offset.left + x) + 'px',
				top:  (offset.top - tip.height() - 10) + 'px',
			})
			if (tip.height() > 30)
				tip.css('left', 0).css('left', x + offset.left - tip.width() - 8)

			reset.f()
			reset = chart.draw(x, 0, w, h, function() {
				ctx.strokeStyle = '#999'
				ctx.fillStyle   = 'rgba(99, 99, 99, .5)'
				ctx.lineWidth   = 1

				ctx.beginPath()
				if (isBar) {
					ctx.moveTo(x, 2.5)
					ctx.lineTo(x+w, 2.5)
					ctx.lineTo(x+w, 47.5)
					ctx.lineTo(x, 47.5)
					ctx.lineTo(x, 2.5)
					ctx.fill()
				}
				else {
					ctx.moveTo(x + ctx.lineWidth/2, 2.5)
					ctx.lineTo(x + ctx.lineWidth/2, 47.5)
					ctx.stroke()
				}
			})
		})
	}

	// Translate country and language names; we do this in JavaScript with Intl,
	// which works fairly well and keeps the backend/database a lot simpler.
	let translate_locations = function() {
		if (!window.Intl || !window.Intl.DisplayNames)
			return

		USER_SETTINGS.widgets.forEach((w, i) => {
			if (w.n !== 'locations' && w.n !== 'languages')
				return
			if (w.s && w.s.key) // Skip "Locations" for a specific region.
				return

			let names = new Intl.DisplayNames([USER_SETTINGS.language], {
				type: (w.n === 'locations' ? 'region' : 'language'),
			})
			let set = function(chart) {
				chart.find('div[data-key]').each((_, e) => {
					if (e.dataset.key.substr(0, 1) === '(') // Skip "(unknown)"
						return
					try {
						let n = names.of(e.dataset.key)
						if (n)
							$(e).find('.col-name .bar-c .cutoff').text(n)
					} catch (exc) {
						// This errors out with a RangeError sometimes, but
						// without details and can't reproduce. Add some more
						// info to see what's going on.
						exc.message = `${exc.message} for type=${w.n}; key=${e.dataset.key}; content=${$(e).find('.col-name .bar-c .cutoff').text()}`
						throw exc
					}
				})
			}

			let chart = $(`.hchart[data-widget=${i}]`)
			set(chart)

			let j = 0
			let t = setInterval(() => {
				j += 1
				if (j > 10)
					clearInterval(t)
				let r = chart.find('.rows')[0]
				if (!r)
					return

				clearInterval(t)
				set(chart)
				let obs = new MutationObserver((mut) => {
					if (mut[0].addedNodes.length === 0 || mut[0].addedNodes[0].classList.contains('detail'))
						return
					obs.disconnect()  // Not strictly needed, but just in case to prevent infinite looping.
					set(chart)
					obs.observe(chart.find('.rows')[0], {childList: true})
				})
				obs.observe(r, {childList: true})
			}, 100)
		})
	}

	// Paginate the main path overview.
	var paginate_pages = function() {
		let sz = $('.pages-list tbody >tr').length
		$('.pages-list >.load-btns .load-less').on('click', function(e) {
			e.preventDefault()
			$(`.pages-list tbody >tr:gt(${sz - 1})`).remove()
			$(this).css('display', 'none')
			$(this).prev('.load-more').css('display', 'inline')
		})

		$('.pages-list >.load-btns .load-more').on('click', function(e) {
			e.preventDefault()

			let btn   = $(this),
				less  = $(this).next('.load-less'),
				pages = $(this).closest('.pages-list')
			let done = paginate_button(btn, () => {
				jQuery.ajax({
					url:  BASE_PATH + '/load-widget',
					data: append_period({
						widget:    pages.attr('data-widget'),
						daily:     $('#daily').is(':checked'),
						exclude:   pages.find('.count-list-pages >tbody >tr').toArray().map((e) => e.dataset.id).join(','),
						max:       get_original_scale(),
					}),
					success: function(data) {
						less.css('display', 'inline')
						pages.find('.count-list-pages >tbody.pages').append(data.html)

						// Update scale in case it's higher than the previous maximum value.
						if (data.max > get_original_scale()) {
							$('.count-list-pages').attr('data-max', data.max)
							$('.count-list-pages').attr('data-scale', data.max)
							$('.count-list-pages .scale').text(data.max)
						}

						draw_all_charts()

						highlight_filter($('#filter-paths').val())
						btn.css('display', data.more ? 'inline-block' : 'none')

						pages.find('.total-display').each((_, t) => {
							$(t).text(format_int(parseInt($(t).text().replace(/[^0-9]/, ''), 10) + data.total_display))
						})

						done()
					},
				})
			})
		})
	}

	// Load references as an AJAX request.
	var load_refs = function() {
		$('.count-list-pages').on('click', '.hchart .load-less', function(e) {
			e.preventDefault()
			let rows = $(this).closest('.hchart').find('.rows'),
				sz   = rows.data('pagesize') || 10
			rows.find(`>div:gt(${sz - 1})`).remove()
			$(this).css('display', 'none')
			$(this).prev('.load-more').css('display', 'inline')
		})

		$('.count-list-pages').on('click', '.load-refs, .hchart .load-more', function(e) {
			e.preventDefault()

			let params = split_query(location.search),
				btn    = $(this),
				less   = btn.next('.load-less'),
				row    = btn.closest('tr'),
				rows   = row.find('.refs .rows'),
				widget = row.closest('.pages-list').attr('data-widget'),
				path   = row.attr('data-id'),
				init   = btn .is('.load-refs'),
				close  = function() {
					var t = $(`tr[data-id=${params['showrefs']}]`)
					t.removeClass('target')
					t.closest('tr').find('.refs').html('')
				}
			if (!rows.data('pagesize'))
				rows.data('pagesize', rows.children().length)

			// Clicked on row that's already open, so close and stop. Don't
			// close anything yet if we're going to load another path, since
			// that gives a somewhat yanky effect (close, wait on xhr, open).
			if (init && params['showrefs'] === path) {
				close()
				return push_query({showrefs: null})
			}

			push_query({showrefs: path})
			let done = paginate_button(btn , () => {
				jQuery.ajax({
					url:   BASE_PATH + '/load-widget',
					data: append_period({
						widget: widget,
						key:    path,
						total:  row.attr('data-count'),
						offset: row.find('.refs .rows>div').length,
					}),
					success: function(data) {
						less.css('display', 'inline')
						row.addClass('target')

						if (init) {
							if (params['showrefs'])
								close()
							row.find('.refs').html(data.html)
						}
						else {
							rows.append($(data.html).find('>div'))
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
		let get_total = () => $('.js-total-utc').text()

		// Paginate the horizontal charts.
		$('.hcharts').on('click', '.load-less', function(e) {
			e.preventDefault()
			let rows = $(this).closest('.hchart').find('.rows'),
				sz   = rows.data('pagesize') || 6
			rows.find(`>div:gt(${sz - 1})`).remove()
			$(this).css('display', 'none')
			$(this).prev('.load-more').css('display', 'inline')
		})

		$('.hcharts').on('click', '.load-more', function(e) {
			e.preventDefault();

			let btn   = $(this),
				less  = btn.next('.load-less'),
				chart = btn.closest('.hchart'),
				key   = chart.attr('data-key'),
				rows  = chart.find('>.rows')
			if (!rows.data('pagesize'))
				rows.data('pagesize', rows.children().length)
			let done = paginate_button($(this), () => {
				jQuery.ajax({
					url:  BASE_PATH + '/load-widget',
					data: append_period({
						widget: chart.attr('data-widget'),
						total:  get_total(),
						key:    key,
						offset: rows.find('div:not(.hchart)').length,
					}),
					success: function(data) {
						less.css('display', 'inline')
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
					url:     BASE_PATH + '/load-widget',
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
		return (s.length === 0 ? location.pathname : ('?' + s.join('&')));
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

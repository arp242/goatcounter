// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

'use strict';

// Prevent a button/link from working while an AJAX request is in progress;
// otherwise smashing a "show more" button will load the same data twice.
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

// Convert "23:45" to "11:45 pm".
var un24 = function(t) {
	if (USER_SETTINGS.twenty_four_hours)
		return t

	var hour = parseInt(t.substr(0, 2), 10)
	if (hour < 12)
		return t + ' am'
	else if (hour == 12)
		return t + ' pm'
	else
		return (hour - 12) + t.substr(2) + ' pm'
}

// Format a number with a thousands separator. https://stackoverflow.com/a/2901298/660921
var format_int = (n) => (n+'').replace(/\B(?=(\d{3})+(?!\d))/g, String.fromCharCode(USER_SETTINGS.number_format))

var months      = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
	days        = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'],
	monthsShort = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
	daysShort   = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

var translate_calendar = function() {
	if (!window.Intl || !window.Intl.DateTimeFormat)
		return
	if (USER_SETTINGS.language.substr(0, 2) === 'en')
		return

	var long  = new Intl.DateTimeFormat(USER_SETTINGS.language, {month: 'long'}),
		short = new Intl.DateTimeFormat(USER_SETTINGS.language, {month: 'short'})
	for (var m=0; m<12; m++) {
		var t = new Date(2017, m, 1, 0, 0, 0)
		months[m]      = long.format(t)
		monthsShort[m] = short.format(t)
	}

	var long  = new Intl.DateTimeFormat(USER_SETTINGS.language, {weekday: 'long'}),
		short = new Intl.DateTimeFormat(USER_SETTINGS.language, {weekday: 'short'})
	for (var d=0; d<7; d++) {
		var t = new Date(2017, 0, d+1, 0, 0, 0)
		days[d]      = long.format(t)
		daysShort[d] = short.format(t)
	}
}

// Format a date according to user configuration.
var format_date = function(date) {
	if (typeof(date) === 'string')
		date = get_date(date)

	var m = date.getMonth() + 1,
		d = date.getDate(),
		items = USER_SETTINGS.date_format.split(/[-/\s]/),
		new_date = []

	// Simple implementation of Go's time format. We only offer the current
	// formatters, so that's all we support:
	//   "2006-01-02", "02-01-2006", "01/02/06", "2 Jan 06", "Mon Jan 2 2006"
	for (var i = 0; i < items.length; i++) {
		switch (items[i]) {
			case '2006': new_date.push(date.getFullYear());                  break;
			case '06':   new_date.push((date.getFullYear() + '').substr(2)); break;
			case '01':   new_date.push(m >= 10 ? m : ('0' + m));             break;
			case '02':   new_date.push(d >= 10 ? d : ('0' + d));             break;
			case '2':    new_date.push(d);                                   break;
			case 'Jan':  new_date.push(monthsShort[date.getMonth()]);        break;
			case 'Mon':  new_date.push(daysShort[date.getDay()]);            break;
		}
	}

	var joiner = '-'
	if (USER_SETTINGS.date_format.indexOf('/') > -1)
		joiner = '/'
	else if (USER_SETTINGS.date_format.indexOf(' ') > -1)
		joiner = ' '
	return new_date.join(joiner)
}

// Format a date as year-month-day.
var format_date_ymd = function(date) {
	if (typeof(date) === 'string')
		return date
	var m = date.getMonth() + 1,
		d = date.getDate()
	return date.getFullYear() + '-' +
		(m >= 10 ? m : ('0' + m)) + '-' +
		(d >= 10 ? d : ('0' + d));
}

// Create Date() object from year-month-day string.
var get_date = function(str) {
	var s = str.split('-')
	return new Date(s[0], parseInt(s[1], 10) - 1, s[2])
}

// Simple z18n-compatible transate.
var T = function(id, params) {
	var str = window.I18N[id]
	if (!str) {
		console.warn(`No translation for ${id}`)
		return id;
	}
	if (typeof params === 'undefined')
		params = {}
	else if (typeof params !== 'object')
		params['__one__'] = params

	return str.replace(/%\((.+?)\)/g, (_, varname) => params[varname] || params['__one__'])
}

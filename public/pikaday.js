/*!
 * Copyright © 2014 David Bushell | BSD & MIT license | https://github.com/Pikaday/Pikaday
 *
 * NOTE: this is a modified version; see git log for details.
 */
(function() {
    'use strict';

    // TODO: these can be removed.
    var
    addEvent    = function(el, e, callback, capture) { el.addEventListener(e, callback, !!capture); },
    removeEvent = function(el, e, callback, capture) { el.removeEventListener(e, callback, !!capture); },

    hasClass = function(el, cn) { return el.classList && el.classList.contains(cn) },
    isArray  = function(obj)    { return (/Array/).test(Object.prototype.toString.call(obj)) },
    isDate   = function(obj)    { return (/Date/).test(Object.prototype.toString.call(obj)) && !isNaN(obj.getTime()) },

	leapYear        = function(year)        { return (year % 4 === 0 && year % 100 !== 0) || year % 400 === 0 },
    getDaysInMonth  = function(year, month) { return [31,(leapYear(year)?29:28),31,30,31,30,31,31,30,31,30,31][month] },
    setToStartOfDay = function(date)        { if (isDate(date)) date.setHours(0,0,0,0) },
    compareDates    = function(a, b)        { return a.getTime() === b.getTime() },

    extend = function(to, from, overwrite) {
        var prop, hasProp;
        for (prop in from) {
            hasProp = to[prop] !== undefined;
            if (hasProp && typeof from[prop] === 'object' && from[prop] !== null && from[prop].nodeName === undefined) {
                if (isDate(from[prop]))
					to[prop] = new Date(from[prop].getTime());
                else if (isArray(from[prop]))
					to[prop] = from[prop].slice(0);
				else
                    to[prop] = extend({}, from[prop], overwrite);
            }
			else
                to[prop] = from[prop];
        }
        return to;
    },

    adjustCalendar = function(calendar) {
        if (calendar.month < 0) {
            calendar.year -= Math.ceil(Math.abs(calendar.month)/12);
            calendar.month += 12;
        }
        if (calendar.month > 11) {
            calendar.year += Math.floor(Math.abs(calendar.month)/12);
            calendar.month -= 12;
        }
        return calendar;
    },

    // defaults and localisation
    defaults = {
        field:         null,        // bind the picker to a form field
        toString:      null,        // Format Date as string.
        parse:         null,        // Create Date object from string.
        firstDay:      0,           // first day of week (0: Sunday, 1: Monday etc)
        minDate:       null,        // the minimum/earliest date that can be selected
        maxDate:       new Date(),  // the maximum/latest date that can be selected
        yearRange:     10,          // number of years either side, or array of upper/lower range
        keyboardInput: true,        // Enable keyboard input
        numberOfMonths: 1,          // how many months are visible

		// when numberOfMonths is used, this will help you to choose where the
		// main calendar will be (default `left`, can be set to `right`)
        // only used for the first display or when a selected date is not visible
        mainCalendar: 'left',

        // internationalization
        isRTL: false,
        i18n: {
			ariaLabel:     'Use the arrow keys to pick a date',
            previousMonth: 'Previous Month',
            nextMonth:     'Next Month',
            weekdays:      ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'],
            weekdaysShort: ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'],
            months:        ['January','February','March','April','May','June','July','August','September','October','November','December'],
        },

        // used internally (don't config outside)
        minYear: 0,
        maxYear: 9999,
        minMonth: undefined,
        maxMonth: undefined,

    },

	// Templating functions to abstract HTML rendering
    renderDayName = function(opts, day, abbr) {
        day += opts.firstDay
        while (day >= 7)
            day -= 7
        return abbr ? opts.i18n.weekdaysShort[day] : opts.i18n.weekdays[day]
    },

    renderDay = function(opts) {
        var arr = [];
        var ariaSelected = 'false';
        if (opts.isEmpty)
            arr.push('is-outside-current-month');
        if (opts.isDisabled)
            arr.push('is-disabled');
        if (opts.isToday)
            arr.push('is-today');
        if (opts.isSelected) {
            arr.push('is-selected');
            ariaSelected = 'true';
        }

        return '<td data-day="' + opts.day + '" class="' + arr.join(' ') + '" aria-selected="' + ariaSelected + '">' +
                 '<button class="pika-button link pika-day" type="button" ' +
                    'data-pika-year="' + opts.year + '" data-pika-month="' + opts.month + '" data-pika-day="' + opts.day + '">' +
                        opts.day +
                 '</button>' +
               '</td>';
    },

    renderRow = function(days, isRTL) {
        return '<tr class="pika-row">' + (isRTL ? days.reverse() : days).join('') + '</tr>';
    },

    renderBody = function(rows) {
        return '<tbody>' + rows.join('') + '</tbody>';
    },

    renderHead = function(opts) {
        var i, arr = [];
        for (i = 0; i < 7; i++) {
            arr.push('<th scope="col"><abbr title="' + renderDayName(opts, i) + '">' + renderDayName(opts, i, true) + '</abbr></th>');
        }
        return '<thead><tr>' + (opts.isRTL ? arr.reverse() : arr).join('') + '</tr></thead>';
    },

    renderTitle = function(instance, c, year, month, refYear, randId) {
        var i, j,
            opts = instance._o,
            isMinYear = year === opts.minYear,
            isMaxYear = year === opts.maxYear,
            html = '<div id="' + randId + '" class="pika-title" role="heading" aria-live="assertive">';

		let months = []
        for (let i = 0; i < 12; i++) {
            months.push('<option value="' + (year === refYear ? i - c : 12 + i - c) + '"' +
                (i === month ? ' selected="selected"': '') +
                ((isMinYear && i < opts.minMonth) || (isMaxYear && i > opts.maxMonth) ? ' disabled="disabled"' : '') + '>' +
                opts.i18n.months[i] + '</option>');
        }

        if (isArray(opts.yearRange)) {
            i = opts.yearRange[0];
            j = opts.yearRange[1] + 1;
        } else {
            i = year - opts.yearRange;
            j = 1 + year + opts.yearRange;
        }

		let years = []
        for (; i < j && i <= opts.maxYear; i++) {
            if (i >= opts.minYear) {
                years.push('<option value="' + i + '"' + (i === year ? ' selected="selected"': '') + '>' + (i) + '</option>');
            }
        }

		html += `
			<div class="pika-label">
				${opts.i18n.months[month]}
				<select class="pika-select pika-select-month" tabindex="-1">${months.join('')}</select>
			</div>
			<div class="pika-label">
				${year}
				<select class="pika-select pika-select-year" tabindex="-1">${years.join('')}</select>
			</div>`

		let prev = true
        if (isMinYear && (month === 0 || opts.minMonth >= month))
            prev = false;

		let next = true
        if (isMaxYear && (month === 11 || opts.maxMonth <= month))
            next = false;

        if (c === 0) {
            html += `<button type="button"
					class="pika-prev link ${prev ? '' : ' is-disabled'}"
					title="${opts.i18n.previousMonth}"
                >◀</button>`
        }
        if (c === (instance._o.numberOfMonths - 1) ) {
            html += `<button type="button"
					class="pika-next link ${next ? '' : ' is-disabled'}"
					title="${opts.i18n.nextMonth}"
				>▶</button>`
        }

        return html += '</div>'
    },

    renderTable = function(opts, data, randId) {
        return '<table cellpadding="0" cellspacing="0" class="pika-table" role="grid" aria-labelledby="' + randId + '">' + renderHead(opts) + renderBody(data) + '</table>';
    },


    // Pikaday constructor
    Pikaday = function(field, options) {
        var self = this,
            opts = self.config(field, options);

        self._onMouseDown = function(e) {
            if (!self._v)
                return;

            var target = e.target;
            if (!target)
                return;

            if (!hasClass(target, 'is-disabled')) {
                if (hasClass(target, 'pika-button') && !hasClass(target, 'is-empty') && !hasClass(target.parentNode, 'is-disabled')) {
                    self.setDate(new Date(
						target.getAttribute('data-pika-year'),
						target.getAttribute('data-pika-month'),
						target.getAttribute('data-pika-day')));

					setTimeout(function() {
						self.hide();
						opts.field.blur();
					}, 100);
                }
                else if (hasClass(target, 'pika-prev'))
                    self.prevMonth();
                else if (hasClass(target, 'pika-next'))
                    self.nextMonth();
            }
            if (!hasClass(target, 'pika-select')) {
                // if this is touch event prevent mouse events emulation
                if (e.preventDefault)
                    e.preventDefault();
				else {
                    e.returnValue = false;
                    return false;
                }
            }
			else
                self._c = true;
        };

		// TODO: never gets called?
        self._onChange = function(e) {
            var target = e.target;
            if (!target)
                return

            if (hasClass(target, 'pika-select-month'))
                self.gotoMonth(target.value)
            else if (hasClass(target, 'pika-select-year'))
                self.gotoYear(target.value)
        };

        self._onKeyChange = function(e) {
            if (!self.isVisible())
                return;

            switch (e.keyCode) {
                case 13:  // <Enter>
                case 27:  // <Esc>
					opts.field.blur()
                    break
                case 37:  // <Left>
                    self.adjustDate('subtract', 1)
                    break
                case 38:  // <Up>
                    self.adjustDate('subtract', 7);
                    break;
                case 39:  // <Right>
                    self.adjustDate('add', 1);
                    break;
                case 40:  // <Down>
                    self.adjustDate('add', 7);
                    break;
            }
        };

        self._parseFieldValue = function() {
            return opts.parse(opts.field.value);
        };

        self._onInputChange = function(e) {
            if (e.firedBy === self)
                return

            var date = self._parseFieldValue()
            if (isDate(date))
              self.setDate(date)

            if (!self._v)
                self.show()
        };

        self._onInputFocus = function() {
            self.show();
        };

        self._onInputClick = function() {
            self.show();
        };

        self._onInputBlur = function() {
            // IE allows pika div to gain focus; catch blur the input field
            var pEl = document.activeElement;
            do {
                if (hasClass(pEl, 'pika-single'))
                    return;
            }
            while ((pEl = pEl.parentNode));

            if (!self._c) {
                self._b = setTimeout(function() {
                    self.hide();
                }, 50);
            }
            self._c = false;
        };

        self._onClick = function(e) {
            var target = e.target,
                pEl = target;
            if (!target)
                return;

            do {
                if (hasClass(pEl, 'pika-single') || pEl === opts.field) {
                    return;
                }
            } while ((pEl = pEl.parentNode));

            if (self._v && target !== opts.field && pEl !== opts.field)
                self.hide();
        };

        self.el = document.createElement('div');
        self.el.className = 'pika-single' + (opts.isRTL ? ' is-rtl' : '');

        addEvent(self.el, 'mousedown', self._onMouseDown, true);
        addEvent(self.el, 'touchend', self._onMouseDown, true);
        addEvent(self.el, 'change', self._onChange);

        if (opts.keyboardInput)
            addEvent(document, 'keydown', self._onKeyChange)

		document.body.appendChild(self.el)
		addEvent(opts.field, 'change', self._onInputChange);

        var defDate = self._parseFieldValue()
        if (isDate(defDate))
			self.setDate(defDate, true)
		else
            self.gotoDate(new Date());

		this.hide();
		self.el.className += ' is-bound';
		addEvent(opts.field, 'click', self._onInputClick);
		addEvent(opts.field, 'focus', self._onInputFocus);
		addEvent(opts.field, 'blur', self._onInputBlur);
    };

    // public Pikaday API
    Pikaday.prototype = {
        // configure functionality
        config: function(field, options) {
			options.field = field
            if (!this._o)
                this._o = extend({}, defaults)
            var opts = extend(this._o, options)

            var nom = parseInt(opts.numberOfMonths, 10) || 1;
            opts.numberOfMonths = nom > 4 ? 4 : nom;

            if (opts.minDate)
                this.setMinDate(opts.minDate);
            if (opts.maxDate)
                this.setMaxDate(opts.maxDate);

            if (isArray(opts.yearRange)) {
                var fallback = new Date().getFullYear() - 10;
                opts.yearRange[0] = parseInt(opts.yearRange[0], 10) || fallback;
                opts.yearRange[1] = parseInt(opts.yearRange[1], 10) || fallback;
            }
			else {
                opts.yearRange = Math.abs(parseInt(opts.yearRange, 10)) || defaults.yearRange;
                if (opts.yearRange > 100)
                    opts.yearRange = 100;
            }

            return opts;
        },

        // return a formatted string of the current selection.
        toString: function() {
            if (!isDate(this._d))
                return ''
            if (this._o.toString)
				return this._o.toString(this._d)
            return this._d.toDateString()
        },

        // return a Date object of the current selection
        getDate: function() {
            return isDate(this._d) ? new Date(this._d.getTime()) : null;
        },

        // set the current selection
        setDate: function(date, noSubmit) {
            if (!date) {
                this._d = null
				this._o.field.value = ''
                return this.draw();
            }

            if (typeof date === 'string')
                date = new Date(Date.parse(date))

            if (!isDate(date))
                return

            this._d = new Date(date.getTime())
            setToStartOfDay(this._d)
            this.gotoDate(this._d)

			this._o.field.value = this.toString()

			if (!noSubmit)
				$(this._o.field).closest('form').trigger('submit')
        },

        // change view to a specific date
        gotoDate: function(date) {
            var newCalendar = true;

            if (!isDate(date))
                return;

            if (this.calendars) {
                var firstVisibleDate = new Date(this.calendars[0].year, this.calendars[0].month, 1),
                    lastVisibleDate = new Date(this.calendars[this.calendars.length-1].year, this.calendars[this.calendars.length-1].month, 1),
                    visibleDate = date.getTime();
                // get the end of the month
                lastVisibleDate.setMonth(lastVisibleDate.getMonth()+1);
                lastVisibleDate.setDate(lastVisibleDate.getDate()-1);
                newCalendar = (visibleDate < firstVisibleDate.getTime() || lastVisibleDate.getTime() < visibleDate);
            }

            if (newCalendar) {
                this.calendars = [{
                    month: date.getMonth(),
                    year: date.getFullYear()
                }];
                if (this._o.mainCalendar === 'right') {
                    this.calendars[0].month += 1 - this._o.numberOfMonths;
                }
            }

            this.adjustCalendars();
        },

        adjustDate: function(sign, days) {
            var day = this.getDate() || new Date();
            var difference = parseInt(days)*24*60*60*1000;

            var newDay;

            if (sign === 'add') {
                newDay = new Date(day.valueOf() + difference);
            } else if (sign === 'subtract') {
                newDay = new Date(day.valueOf() - difference);
            }

            this.setDate(newDay);
        },

        adjustCalendars: function() {
            this.calendars[0] = adjustCalendar(this.calendars[0]);
            for (var c = 1; c < this._o.numberOfMonths; c++) {
                this.calendars[c] = adjustCalendar({
                    month: this.calendars[0].month + c,
                    year: this.calendars[0].year
                });
            }
            this.draw();
        },

        gotoToday: function() {
            this.gotoDate(new Date());
        },

        // change view to a specific month (zero-index, e.g. 0: January)
        gotoMonth: function(month) {
            if (!isNaN(month)) {
                this.calendars[0].month = parseInt(month, 10);
                this.adjustCalendars();
            }
        },

        nextMonth: function() {
            this.calendars[0].month++;
            this.adjustCalendars();
        },

        prevMonth: function() {
            this.calendars[0].month--;
            this.adjustCalendars();
        },

        // change view to a specific full year (e.g. "2012")
        gotoYear: function(year) {
            if (!isNaN(year)) {
                this.calendars[0].year = parseInt(year, 10);
                this.adjustCalendars();
            }
        },

        // change the minDate
        setMinDate: function(value) {
            if(value instanceof Date) {
                setToStartOfDay(value);
                this._o.minDate = value;
                this._o.minYear  = value.getFullYear();
                this._o.minMonth = value.getMonth();
            }
			else {
                this._o.minDate = defaults.minDate;
                this._o.minYear  = defaults.minYear;
                this._o.minMonth = defaults.minMonth;
            }

            this.draw();
        },

        // change the maxDate
        setMaxDate: function(value) {
            if(value instanceof Date) {
                setToStartOfDay(value);
                this._o.maxDate = value;
                this._o.maxYear = value.getFullYear();
                this._o.maxMonth = value.getMonth();
            } else {
                this._o.maxDate = defaults.maxDate;
                this._o.maxYear = defaults.maxYear;
                this._o.maxMonth = defaults.maxMonth;
            }

            this.draw();
        },

        // refresh the HTML
        draw: function(force) {
            if (!this._v && !force)
                return;

            var opts = this._o,
                minYear = opts.minYear,
                maxYear = opts.maxYear,
                minMonth = opts.minMonth,
                maxMonth = opts.maxMonth,
                html = '',
                randId;

            if (this._y <= minYear) {
                this._y = minYear;
                if (!isNaN(minMonth) && this._m < minMonth)
                    this._m = minMonth;
            }
            if (this._y >= maxYear) {
                this._y = maxYear;
                if (!isNaN(maxMonth) && this._m > maxMonth)
                    this._m = maxMonth;
            }

            for (var c = 0; c < opts.numberOfMonths; c++) {
                randId = 'pika-title-' + Math.random().toString(36).replace(/[^a-z]+/g, '').substr(0, 2);
                html += '<div class="pika-lendar">' + renderTitle(this, c, this.calendars[c].year, this.calendars[c].month, this.calendars[0].year, randId) + this.render(this.calendars[c].year, this.calendars[c].month, randId) + '</div>';
            }

            this.el.innerHTML = html;

            if (opts.field.type !== 'hidden')
				setTimeout(function() { opts.field.focus() }, 1);

            // let the screen reader user know to use arrow keys
            opts.field.setAttribute('aria-label', opts.i18n.ariaLabel);
        },

        adjustPosition: function() {
            var field, pEl, width, height, viewportWidth, viewportHeight, scrollTop,
				left, top, clientRect;

            this.el.style.position = 'absolute';

            field          = this._o.field;
            pEl            = field;
            width          = this.el.offsetWidth;
            height         = this.el.offsetHeight;
            viewportWidth  = window.innerWidth || document.documentElement.clientWidth;
            viewportHeight = window.innerHeight || document.documentElement.clientHeight;
            scrollTop      = window.pageYOffset || document.body.scrollTop || document.documentElement.scrollTop;

            if (typeof field.getBoundingClientRect === 'function') {
                clientRect = field.getBoundingClientRect();
                left = clientRect.left + window.pageXOffset;
                top = clientRect.bottom + window.pageYOffset;
            }
			else {
                left = pEl.offsetLeft;
                top  = pEl.offsetTop + pEl.offsetHeight;
                while ((pEl = pEl.offsetParent)) {
                    left += pEl.offsetLeft;
                    top  += pEl.offsetTop;
                }
            }

            this.el.style.left = Math.max(left, 0) + 'px';
            this.el.style.top =  Math.max(top, 0) + 'px';
			this.el.classList.add('left-aligned')
			this.el.classList.add('bottom-aligned')
        },

        // render HTML for a particular month
        render: function(year, month, randId) {
            var opts   = this._o,
                now    = new Date(),
                days   = getDaysInMonth(year, month),
                before = new Date(year, month, 1).getDay(),
                data   = [],
                row    = [];
            setToStartOfDay(now);
            if (opts.firstDay > 0) {
                before -= opts.firstDay;
                if (before < 0)
                    before += 7
            }
            var previousMonth       = month === 0 ? 11 : month - 1,
                nextMonth           = month === 11 ? 0 : month + 1,
                yearOfPreviousMonth = month === 0 ? year - 1 : year,
                yearOfNextMonth     = month === 11 ? year + 1 : year,
                daysInPreviousMonth = getDaysInMonth(yearOfPreviousMonth, previousMonth);

            var cells = days + before,
                after = cells;
            while (after > 7)
                after -= 7;
            cells += 7 - after;

            for (var i=0, r=0; i<cells; i++) {
                var day          = new Date(year, month, 1 + (i - before)),
                    isSelected   = isDate(this._d) ? compareDates(day, this._d) : false,
                    isToday      = compareDates(day, now),
                    isEmpty      = i < before || i >= (days + before),
                    dayNumber    = 1 + (i - before),
                    monthNumber  = month,
                    yearNumber   = year,
                    isDisabled   = (opts.minDate && day < opts.minDate) ||
                                   (opts.maxDate && day > opts.maxDate);

                if (isEmpty) {
                    if (i < before) {
                        dayNumber = daysInPreviousMonth + dayNumber;
                        monthNumber = previousMonth;
                        yearNumber = yearOfPreviousMonth;
                    }
					else {
                        dayNumber = dayNumber - days;
                        monthNumber = nextMonth;
                        yearNumber = yearOfNextMonth;
                    }
                }

                row.push(renderDay({
					day:          dayNumber,
					month:        monthNumber,
					year:         yearNumber,
					isSelected:   isSelected,
					isToday:      isToday,
					isDisabled:   isDisabled,
					isEmpty:      isEmpty,
				}))

                if (++r === 7) {
                    data.push(renderRow(row, opts.isRTL));
                    row = [];
                    r = 0;
                }
            }
            return renderTable(opts, data, randId);
        },

        isVisible: function() {
            return this._v;
        },

        show: function() {
            if (!this.isVisible()) {
                this._v = true;
                this.draw();
                this.el.classList.remove('is-hidden');
				addEvent(document, 'click', this._onClick);
				this.adjustPosition();
            }
        },

        hide: function() {
            var v = this._v;
            if (v !== false) {
                removeEvent(document, 'click', this._onClick);

                this.el.style.position = 'static'; // reset
                this.el.style.left = 'auto';
                this.el.style.top = 'auto';

                this.el.classList.add('is-hidden');
                this._v = false;
            }
        },
    };

    window.Pikaday = Pikaday;
}());

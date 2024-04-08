// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

(function() {
	'use strict';

	window.charty = function(ctx, data, opt) {
		// Width and height attributes need to be set explicitly as attributes;
		// one "CSS pixel" may not correspond with one "physical pixe", for
		// example when zooming (either in browser or OS, e.g. for high-DPI
		// displays), so account for that with devicePixelRatio.
		let dpr = Math.max(1, window.devicePixelRatio || 1)
		ctx.canvas.width  = ctx.canvas.clientWidth  * dpr
		ctx.canvas.height = ctx.canvas.clientHeight * dpr
		ctx.scale(dpr, dpr)

		opt.line = Object.assign({width: 2, color: '#f00', fill: '#fdd'}, opt.line)
		opt.bar  = Object.assign({color: '#f00'}, opt.bar)
		opt      = Object.assign({mode: 'line', max: 0, pad: 2, background: style('bg'), grid: [2.5, 22.5, 47.5]}, opt)

		if (opt.max === 0)
			opt.max = data.reduce((a, b) => b > a ? b : a)
		let relData = data.map((n) => n / opt.max * 100)

		let pad      = (opt.pad + 1.5) / dpr,  // .5 for alignment, and 1 for border.
			barWidth = (ctx.canvas.width - pad) / dpr / relData.length
		if (opt.mode === 'line')
			barWidth += barWidth / relData.length - pad*1.5 / relData.length
		else
			barWidth -= pad / relData.length

		// Fill background so getContext('2d', {alpha: false}) works.
		ctx.beginPath()
		ctx.fillStyle = opt.background
		ctx.fillRect(0, 0, ctx.canvas.width, ctx.canvas.height)

		let cWidth  = ctx.canvas.width  / dpr,
			cHeight = ctx.canvas.height / dpr

		if (opt.grid)
			draw_grid(ctx, cWidth, cHeight, pad, opt.grid)
		if (opt.mode === 'bar')
			draw_barchart(ctx, relData, barWidth, cWidth, cHeight, pad, opt.bar)
		else
			draw_linechart(ctx, relData, barWidth, cWidth, cHeight, pad, opt.line)

		let self = {}

		let stop = () => {
			window.removeEventListener('resize', r)
			self.unbind_mouse()
		}

		// Redraw on resize.
		let t
		let r = (e) => {
			clearTimeout(t)
			t = setTimeout(() => {
				stop()
				charty(ctx, data, opt)
			}, 200)
		}
		window.addEventListener('resize', r)

		self = {
			ctx:      () => ctx,
			barWidth: () => barWidth,
			pad:      () => pad,

			unbind_mouse: () => {},

			// Unbind all events.
			stop: stop,

			// Handle mouse events. Callback is called with the following
			// arguments:
			//
			// i      → index in data.
			// x, y   → start coördinate of this bar.
			// w, h   → width + height of this bar.
			// ev     → event type as string: 'enter', 'leave', 'move'.
			// offset → position of this chart, left + top.
			mouse: (cb) => handle_mouse(self, ctx.canvas, relData, barWidth, pad, cb),

			// Draw something at the given coördinates in the callback, returns
			// a function to reset to the previous value.
			draw: (x, y, width, height, cb) => {
				x *= Math.max(window.devicePixelRatio, 1)
				width *= Math.max(window.devicePixelRatio, 1)
				let save = ctx.getImageData(x-4, y-4, width+8, height+8)
				cb()
				return {x: x, y: y, f: () => ctx.putImageData(save, x-4, y-4)}
			},
		}

		if (opt.done)
			opt.done(self)

		return self
	}

	// Handle mouse events.
	let handle_mouse = function(self, canvas, data, barWidth, pad, cb) {
		let f = function(e) {
			let ev     = {mousemove: 'move', mouseenter: 'enter', mouseleave: 'leave'}[e.type],
				offset = get_offset(this),
				mouseX = e.clientX - offset.left,
				i      = Math.round((mouseX + barWidth/2) / barWidth) - 1,
				x      = barWidth * i + pad,
				y      = data[i]

			// 591 598 3.5
			// console.log(mouseX, canvas.width, pad)
			// || mouseX < pad || mouseX >= canvas.width - pad*2))
			if (ev !== 'leave' && (typeof y === 'undefined'))
				return
			cb(i, x, y, barWidth, canvas.height - pad, offset, ev)
		}

		canvas.addEventListener('mousemove',  f)
		canvas.addEventListener('mouseenter', f)
		canvas.addEventListener('mouseleave', f)

		self.unbind_mouse = () => {
			canvas.removeEventListener('mousemove',  f)
			canvas.removeEventListener('mouseenter', f)
			canvas.removeEventListener('mouseleave', f)
		}
	}

	// Get offset relative to this element.
	let get_offset = function(elem) {
		let rect    = elem.getBoundingClientRect(),
			doc     = elem.ownerDocument,
			docElem = doc.documentElement,
			win     = doc.defaultView
		return {
			top:  rect.top  + win.pageYOffset - docElem.clientTop,
			left: rect.left + win.pageXOffset - docElem.clientLeft
		}
	}

	// Draw gridlines and borders.
	let draw_grid = function(ctx, cWidth, cHeight, pad, grid) {
		ctx.lineWidth   = 1
		ctx.strokeStyle = style('chart-grid')

		grid.forEach((g) => {
			ctx.beginPath()
			ctx.moveTo(pad, g)
			ctx.lineTo(cWidth-pad , g)
			ctx.stroke()
		})

		ctx.beginPath()
		ctx.lineTo(pad, pad)         // Left border.
		ctx.lineTo(pad, cHeight-pad)
		ctx.stroke()
		ctx.beginPath()
		ctx.lineTo(cWidth-pad, pad)  // Right border.
		ctx.lineTo(cWidth-pad, cHeight-pad)
		ctx.stroke()
	}

	// Draw bar chart.
	let draw_barchart = function(ctx, data, barWidth, cWidth, cHeight, pad, opt) {
		ctx.fillStyle   = opt.color
		ctx.strokeStyle = opt.color
		ctx.lineWidth   = 1

		ctx.beginPath()
		let x = pad
		data.forEach((p) => {
			let y = (cHeight + pad - p/2) * (1 - pad/cHeight*2)
			if (p === 0)
				y = cHeight - pad

			ctx.lineTo(
				Math.round(x)+.5,
				Math.round(y)+.5)
			ctx.lineTo(
				Math.round(x + barWidth)+.5,
				Math.round(y)+.5)
			x += barWidth
		})

		ctx.lineTo(x + 1, cHeight - pad + 1)
		ctx.lineTo(pad,   cHeight - pad + 1)

		ctx.stroke()
		ctx.fill()
	}

	// Draw linechart.
	let draw_linechart = function(ctx, data, barWidth, cWidth, cHeight, pad, opt) {
		ctx.strokeStyle = opt.color
		ctx.fillStyle   = opt.fill
		ctx.lineWidth   = opt.width
		ctx.miterLimit  = 1

		let trace = function(f) {
			let x = pad
			ctx.beginPath()
			data.forEach((p) => {
				let y = (cHeight + pad - p/2) * (1 - pad/cHeight*2)
				ctx.lineTo(Math.round(x), y)
				x += barWidth
			})
		}

		// Draw the "fill" bottom first to ensure the line gets drawn on top.
		if (opt.fill) {
			trace()
			ctx.lineTo(cWidth - pad, cHeight - pad)
			ctx.lineTo(pad, cHeight - pad)
			ctx.fill()
		}
		trace()
		ctx.stroke()
	}
})()

let helpIndex = window.location.pathname.lastIndexOf('/help');
let basePath = window.location.pathname.slice(0, helpIndex === -1 ? 0 : helpIndex)
document.querySelector('select').addEventListener('change', function(e) {
	window.location = basePath + '/help/' + this.value
})
document.querySelector('.show-contact').addEventListener('click', function(e) {
	e.preventDefault()
	document.querySelector('.contact').style.display = ''
	this.parentNode.removeChild(this)
	window.location.hash = '#scroll-target'
})

let expand = document.querySelector('.expand')
if (expand)
	expand.addEventListener('click', function(e) {
		e.preventDefault()
		let a = this,
			d = a.dataset.expand
		if (d)
			document.querySelectorAll(d).forEach((elem) => {
				if (elem.style.display === 'block') {
					elem.style.display = 'none'
					a.classList.remove('active')
					return
				}
				elem.style.display = 'block'
				a.classList.add('active')
			})
	})

if (window.location.pathname === basePath + '/help/visitor-counter') {
	var t = setInterval(function() {
		if (window.goatcounter && window.goatcounter.visit_count) {
			clearInterval(t)
			window.goatcounter.visit_count({append: '#vc-html', type: 'html'})
			window.goatcounter.visit_count({append: '#vc-png',  type: 'png'})
			window.goatcounter.visit_count({append: '#vc-svg',  type: 'svg'})
			window.goatcounter.visit_count({append: '#vc-html', type: 'html', path: 'TOTAL'})
			window.goatcounter.visit_count({append: '#vc-png',  type: 'png',  path: 'TOTAL'})
			window.goatcounter.visit_count({append: '#vc-svg',  type: 'svg',  path: 'TOTAL'})
		}
	}, 100)
}

// Add copy button to <pre>.
document.querySelectorAll('pre').forEach(function(pre) {
	// if (pre.clientHeight > pre.scrollHeight - 6)
	// 	return

	var btn = document.createElement('a')
	btn.href        = '#'
	btn.className   = 'pre-copy'
	btn.style.right = (pre.offsetWidth - pre.clientWidth - 2) + 'px'
	btn.innerHTML   = '📋 Copy'

	btn.addEventListener('click', function(e) {
		e.preventDefault()

		var t = document.createElement('textarea')
		t.value = pre.innerText
		t.style.position = 'absolute'
		document.body.appendChild(t)

		t.select()
		t.setSelectionRange(0, pre.innerText.length)
		document.execCommand('copy')
		document.body.removeChild(t)

		btn.innerHTML = '📋 Done'
		setTimeout(function() { btn.innerHTML = '📋 Copy' }, 1000)
	}, false)

	//pre.parentNode.appendChild(btn)

	// TODO: can probably un-jQuery this.
	var wrap = $('<div class="pre-copy-wrap">').html($(pre).clone())
	$(pre).replaceWith(wrap)
	wrap.prepend(btn)

	var prev = wrap.prev()
	if (prev.is('p'))
		prev.css('padding-right', '6em')
})

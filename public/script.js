// Get Array of HTMLElements from querySelectorAll().
var queryAll = function (q) {
	var nodes = document.querySelectorAll(q),
		arr = [];

	arr.length = nodes.length;
	for (var i = 0; i < nodes.length; i++) {
		if (!(nodes[i] instanceof HTMLElement)) {
			arr.length--;
			continue;
		}
		arr[i] = nodes[i];
	}

	return arr;
};

// Get HTMLElement from querySelector().
var query = function (q) {
	var n = document.querySelector(q);
	if (!n || !(n instanceof HTMLElement)) {
		return null;
	}
	return n;
};

var quote = function(s) {
	return s
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&apos;');
};

var init = function() {
};

if (document.readyState === 'complete')
	init();
else
	window.addEventListener('load', init, false);

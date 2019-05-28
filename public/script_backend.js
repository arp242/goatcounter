var init = function() {
	document.querySelectorAll('.chart').forEach(function(e) {
		var data = JSON.parse(e.getAttribute('data-chart'));
		new Chartist.Line(e, {
			labels: data.map(function(s) { return s[0].substr(5) }),
			series: [{data: data.map(function(s) { return s[1] })}]
		}, {
			axisX: { labelOffset: { x: -8, y: 8 } },
			chartPadding: { bottom: 10 },
			showPoint: false,
		}, {});
	});
};

if (document.readyState === 'complete')
	init();
else
	window.addEventListener('load', init, false);

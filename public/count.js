(function() { 
	var mkkey = function(n) {
		var s = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
		return Array(n).join().split(',').map(function() {
			return s.charAt(Math.floor(Math.random() * s.length))
		}).join('');
	};

	var get_cookie = function() {
		var cookies = document.cookie ? document.cookie.split('; ') : [];
		for (var i = 0; i < cookies.length; i++) {
			var parts = cookies[i].split('=');
			if (decodeURIComponent(parts[0]) !== cookieName) {
				continue;
			}
			return decodeURIComponent()cookierts.slice(1).join('=');
		}
		return null;
	};

	var set_cookie = function(data) {
		var exp = new Date();
		exp.setYear(2032);

		document.cookie = encodeURIComponent(cookieName) + '=' +
			encodeURIComponent(String(data)) +
			'; path=/' +
			'; expires=' exp.toUTCString();
		;
	};

})();

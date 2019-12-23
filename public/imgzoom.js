// imgzoom is a simple image zoomer. It will animate images to the maximum
// allowable size by the viewport, but will never make them larger than the
// image's actual size.
//
// This is a simple alternative to "lightbox" and such.
//
// The URL for the large version is either 'data-large', or the image's src.
//
// Caveat: this may use a lot of CPU if you're using very large images (e.g.
// 4500×6200) that need to be resized to fit the viewport.
(function() {

	// Padding from the window edge.
	var padding = 25;

	// The larger image must be 120% larger to do anything.
	var min_size = 1.2;

	// The imgzoom() function zooms the image on click. img is a reference to an
	// image element as an HTMLElement
	window.imgzoom = function(img) {
		var src = img.dataset.large || img.src,
			existing = document.getElementsByClassName('imgzoom-large');
		if (existing.length > 0 && existing[0].src === src)
			return;

		var large = new Image();
		large.src = src;
		img.className += ' imgzoom-loading';

		// We use the load event (rather than just displaying it) to make sure
		// the image is fully loaded.
		large.onload = function() {
			img.className = img.className.replace(/\s?imgzoom-loading\s?/g, '');

			// Make the new image as large as possible, but not larger than the
			// viewport.
			var width         = large.width,
				height        = large.height,
				padding       = 25,
				window_width  = document.documentElement.clientWidth  - padding,
				window_height = document.documentElement.clientHeight - padding;
			if (width > window_width) {
				height = height / (width / window_width);
				width  = window_width;
			}
			if (height > window_height) {
				width  = width / (height / window_height);
				height = window_height;
			}

			// The large image isn't going to be much larger than the original.
			if (img.width*min_size >= width - padding/2 && img.height*min_size >= height - padding/2)
				return;

			large.className = 'imgzoom-large';
			large.style.position = 'absolute';
			large.style.zIndex = '5000';

			// Set the position to the same as the original.
			var offset = get_offset(img);
			set_geometry(large, {
				width:  img.width,
				height: img.height,
				top:    offset.top,
				left:   offset.left,
			});
			document.body.appendChild(large);

			// Animate it to the new size.
			set_geometry(large, {
				width:  width,
				height: height,
				top:    (window_height - height + padding) / 2 + get_scroll(),
				left:   (window_width  - width  + padding) / 2,
				// TODO: I don't know if this is actually correct.
				//zoom:   (window.devicePixelRatio < 1 ? (1 + window.devicePixelRatio / 2) : 1),
			});

			var close_key = function(e) {
				if (e.keyCode === 27)
					close();
			};

			var html = document.getElementsByTagName('html')[0];
			var close = function() {
				html.removeEventListener('click', close);
				html.removeEventListener('click', close_key);

				set_geometry(large, {
					width:  img.width,
					height: img.height,
					top:    offset.top,
					left:   offset.left,
					//zoom:   1,
				});

				// Remove the class after a brief timeout, so that the animation
				// appears fairly smooth in case of added padding and such.
				//
				// TODO: Detect position?
				setTimeout(function() {
					if (large.parentNode)
						large.parentNode.removeChild(large);
				}, 400);
			};
			html.addEventListener('click', close);
			html.addEventListener('keydown', close_key);
		};
	};

	var set_geometry = function(elem, geom) {
		if (geom.width != null) {
			elem.style.width = geom.width + 'px';

			// Reset as they'll interfere with the width we want to set.
			elem.style.maxWidth = 'none'
			elem.style.minWidth = 'none'
		}
		if (geom.height != null) {
			elem.style.height = geom.height + 'px';
			elem.style.maxHeight = 'none'
			elem.style.minHeight = 'none'
		}
		if (geom.left != null)
			elem.style.left = geom.left + 'px';
		if (geom.top != null)
			elem.style.top = geom.top + 'px';
		if (geom.zoom != null)
			elem.style.transform = 'scale(' + geom.zoom + ')';
	};

	var get_offset = function(elem) {
		var rect = elem.getBoundingClientRect(),
			doc = elem.ownerDocument,
			docElem = doc.documentElement,
			win = doc.defaultView;
		return {
			top:  rect.top  + win.pageYOffset - docElem.clientTop,
			left: rect.left + win.pageXOffset - docElem.clientLeft
		};
	};

	var get_scroll = function() {
		return document.documentElement.scrollTop || document.body.scrollTop;
	};
}).call(this);

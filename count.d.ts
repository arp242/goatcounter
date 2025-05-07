interface Window {
	/**
	 * GoatCounter JavaScript API
	 *
	 * @see https://www.goatcounter.com/help/js
	 */
	goatcounter:
		| (GoatCounter.Settings &
				GoatCounter.DataParameters &
				GoatCounter.Methods)
		| undefined;
}

declare namespace GoatCounter {
	/**
	 * Settings that can be defined via:
	 *
	 * - `window.goatcounter` object, declared _before_ loading `count.js`
	 * - `<script>` tag `data-goatcounter-settings` attribute (will override settings in `window.goatcounter`)
	 */
	interface Settings {
		/** Don’t do anything on page load, for cases where you want to call `count()` manually. Also won’t bind events. */
		no_onload?: boolean;
		/** Don’t bind events. */
		no_events?: boolean;
		/** Allow requests from local addresses (`localhost`, `192.168.0.0`, etc.) for testing the integration locally. */
		allow_local?: boolean;
		/** Allow requests when the page is loaded in a frame or iframe. */
		allow_frame?: boolean;
		/** Customize the endpoint for sending pageviews to (overrides the URL in `data-goatcounter`). Only useful if you have `no_onload`. */
		endpoint?: string;
	}

	interface DataParameters {
		/**
		 * Page path (without domain) or event name.
		 *
		 * Default is the value of `<link rel="canonical">` if it exists, or
		 * `location.pathname + location.search`.
		 *
		 * Alternatively, a callback that takes the default value and
		 * returns a new value. No pageview is sent if the callback returns
		 * `null`.
		 *
		 * @see https://www.goatcounter.com/help/modify
		 */
		path?: string | ((defaultValue: string) => string | null);

		/**
		 * Human-readable title.
		 *
		 * Default is `document.title`.
		 *
		 * Alternatively, a callback that takes the default value and
		 * returns a new value.
		 */
		title?: string | ((defaultValue: string) => string);

		/**
		 * Where the user came from; can be an URL (`https://example.com`)
		 * or any string (`June Newsletter`).
		 *
		 * Default is the `Referer` header.
		 *
		 * Alternatively, a callback that takes the default value and
		 * returns a new value.
		 */
		referrer?: string | ((defaultValue: string) => string);

		/**
		 * Treat the path as an event, rather than a URL.
		 */
		event?: boolean;
	}

	interface Methods {
		/**
		 * Send a pageview or event to GoatCounter.
		 *
		 * @param vars merged in to the global `window.goatcounter`, if it exists
		 */
		count(vars?: DataParameters): void;

		/**
		 * Get URL to send to the server.
		 *
		 * @param vars merged in to the global `window.goatcounter`, if it exists
		 */
		url(vars?: DataParameters): string | undefined;

		/**
		 * Determine if this request should be filtered.
		 *
		 * This will filter some bots, pre-render requests, frames (unless
		 * `allow_frame` is set), and local requests (unless `allow_local`
		 * is set).
		 *
		 * @returns string with the reason or `false`
		 */
		filter(): string | false;

		/**
		 * Bind a click event to every element with `data-goatcounter-click`.
		 *
		 * Called on page load unless `no_onload` or `no_events` is set. You
		 * may need to call this manually if you insert elements after the
		 * page loads.
		 *
		 * @see https://www.goatcounter.com/help/events
		 */
		bind_events(): void;

		/**
		 * Get a single query parameter from the current page’s URL.
		 *
		 * @returns `undefined` if the parameter doesn’t exist.
		 */
		get_query(name: string): string | undefined;

		/**
		 * Display a page’s view count by appending an HTML document or image.
		 *
		 * @see https://www.goatcounter.com/help/visitor-counter
		 */
		visit_count(opt?: VisitCountOptions): void;
	}

	interface VisitCountOptions {
		/** HTML selector to append to, can use CSS selectors as accepted by `querySelector()`. Default is `body`. */
		append?: string;
		/** Output type. Default is `html`. */
		type?: "html" | "svg" | "png";
		/** Path to display; normally this is detected from the URL, but you can override it. */
		path?: string;
		/** Don’t display “by GoatCounter” branding */
		no_branding?: boolean;
		/** HTML attributes to set or override for the element, only when type is `html`. */
		attr?: Record<string, string>;
		/** Extra CSS styling for HTML or SVG; only when `type` is `html` or `svg`. */
		style?: string;
		/** Start date; default is to include everything. As `year-month-day` or `week`, `month`, `year` for this period ago. */
		start?: string;
		/** End date; default is to include everything. As `year-month-day`. */
		end?: string;
	}
}

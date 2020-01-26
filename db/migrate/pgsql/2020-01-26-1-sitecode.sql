begin;
	insert into updates (subject, created_at, show_at, body) values (
		'Simpler site code', now(), now(),
		'<p>The <a href="/settings#tab-site-code">site code</a> is now
			significantly simpler. The old one will still work, but it’s recommended
			to use the new one.</p>

		<p>The page now also documents how to integrate GoatCounter on your site without JavaScript.</p>
			');

	insert into version values ('2020-01-26-1-sitecode');
commit;

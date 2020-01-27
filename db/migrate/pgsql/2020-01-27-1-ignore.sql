begin;
	insert into updates (subject, created_at, show_at, body) values (
		'New setting: ignoring your own views', now(), now(),
		'<p>There is now a setting to ignore your own views based on IP address</p>

		<p>Note you can also do this client-side if you prefer; there is an
			example in the <a href="/settings#tab-site-code">site code</a> for
			this now as well (“skip own views”).</p>
	');

	insert into version values ('2020-01-27-1-ignore');
commit;


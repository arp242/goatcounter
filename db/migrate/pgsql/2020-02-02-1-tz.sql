begin;
	insert into updates (subject, created_at, show_at, body) values (
		'New setting: timezone', now(), now(),
		'<p>The charts can now be displayed in the local timezone; you can
			change the timezone in the site settings.</p>
		<p>The timezone should be automatically set to your browser’s local
			timezone.</p>
	');

	insert into version values ('2020-02-02-1-tz');
commit;

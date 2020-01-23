begin;
	insert into updates (subject, created_at, show_at, body) values (
		'New setting: data retention', now(), now(),
		'<p>You can now limit the amount of time GoatCounter keeps data in your site settings.</p>');

	insert into version values ('2020-01-23-2-retention');
commit;

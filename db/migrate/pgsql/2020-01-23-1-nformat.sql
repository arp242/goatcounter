begin;
	insert into updates (subject, created_at, show_at, body) values (
		'New setting: thousands separator', now(), now(),
		'<p>You can now choose which thousands separators is used to format
		numbers in your site’s settings. The default is still a thin space as
		before, as that’s the most universal format.</p>');

	update sites set settings=substr(settings, 0, length(settings)) || ', "number_format": 8239}';
	insert into version values ('2020-01-23-1-nformat');
commit;

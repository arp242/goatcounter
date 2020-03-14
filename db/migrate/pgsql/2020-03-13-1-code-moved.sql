begin;
	insert into updates (subject, created_at, show_at, body) values (
		'Site code moved', now(), now(),
		'<p>Just a little heads-up that the “site code” is now its own page
		linked in the top menu, instead of a tab in the settings page. This will
		allow permalinks to sections, which was tricky on the tab page because
		permalinks are already used there for the tabs.</p>');

	insert into version values ('2020-03-13-1-code-moved');
commit;

begin;
	insert into updates (subject, created_at, show_at, body) values (
		'New setting: domain (for linking)', now(), now(),
		'<p>You can now fill in you site’s domain in the settings; this will allow linking the path directly from the overview.</p>');

	alter table hits drop column domain;
	alter table hits add column event int default 0;
	alter table sites add column link_domain varchar not null default '';

	insert into version values ('2020-01-24-2-domain');
commit;

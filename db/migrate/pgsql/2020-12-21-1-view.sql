begin;
	update sites set settings = jsonb_set(settings, '{views}',
		'[{"name": "default", "period": "week"}]', true);
	insert into version values('2020-12-21-1-view');
commit;

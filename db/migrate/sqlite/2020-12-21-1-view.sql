begin;
	update sites set settings = json_set(settings, '$.views',
		json('[{"name": "default", "period": "week"}]'));
	insert into version values('2020-12-21-1-view');
commit;

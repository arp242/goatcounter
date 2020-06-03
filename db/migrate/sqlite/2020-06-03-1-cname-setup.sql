begin;
	alter table sites add column cname_setup_at timestamp default null
		check(cname_setup_at = strftime('%Y-%m-%d %H:%M:%S', cname_setup_at));

	update sites set cname_setup_at=datetime() where cname is not null;

	insert into version values('2020-06-03-1-cname-setup');
commit;

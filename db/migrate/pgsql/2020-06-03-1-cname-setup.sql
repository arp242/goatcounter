begin;
	alter table sites add column cname_setup_at timestamp default null;
	update sites set cname_setup_at=now() where cname is not null;

	insert into version values('2020-06-03-1-cname-setup');
commit;

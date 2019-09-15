begin;
	alter table hits add column
		size           varchar        not null default '0,0';
	insert into version values ('2019-09-15-1-size');
commit;

begin;
	alter table hits
		add column count_ref varchar not null default '';
	insert into version values ('2019-12-10-2-count_ref');
commit;

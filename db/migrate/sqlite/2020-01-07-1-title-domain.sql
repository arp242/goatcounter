begin;
	alter table hits add column title varchar;
	alter table hits add column domain varchar;

	insert into version values ('2020-01-07-1-title-domain');
commit;

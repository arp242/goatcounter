begin;
	alter table hits add column title varchar not null default '';
	alter table hits add column domain varchar not null default '';

	insert into version values ('2020-01-07-1-title-domain');
commit;

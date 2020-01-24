begin;
	alter table browser_stats drop column mobile;
	insert into version values ('2020-01-24-1-rm-mobile');
commit;


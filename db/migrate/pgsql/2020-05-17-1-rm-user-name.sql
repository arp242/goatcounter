begin;
	alter table users drop column name;
	alter table sites drop column name;
	insert into version values ('2020-05-17-1-rm-user-name');
commit;

begin;
	alter table users drop column preferences;
	alter table users drop column state;

	insert into version values ('2019-09-17-1-old-user');
commit;


begin;
	alter table hits add column id serial primary key;
	insert into version values ('2020-02-06-1-hitsid');
commit;

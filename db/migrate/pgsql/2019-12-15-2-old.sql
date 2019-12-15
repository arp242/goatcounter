begin;
	alter table hit_stats drop column updated_at;
	alter table hit_stats drop column created_at;
	insert into version values ('2019-12-15-2-old');
commit;

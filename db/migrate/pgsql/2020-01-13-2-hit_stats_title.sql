begin;
	alter table hit_stats add column title varchar not null default '';
	insert into version values ('2020-01-13-2-hit_stats_title');
commit;

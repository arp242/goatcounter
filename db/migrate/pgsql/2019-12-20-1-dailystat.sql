begin;
	alter table hit_stats
		add column total integer not null default 0;

	insert into version values ('2019-12-20-1-dailystat');
commit;

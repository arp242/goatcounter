begin;
	alter table hit_stats      add column event integer default 0;
	alter table browser_stats  add column event integer default 0;
	alter table location_stats add column event integer default 0;
	alter table size_stats     add column event integer default 0;
	alter table ref_stats      add column event integer default 0;

	insert into version values ('2020-04-06-1-event');
commit;

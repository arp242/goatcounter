begin;
	update hits       set path=substr(path, 2) where event=1 and path like '/%';
	update hit_counts set path=substr(path, 2) where event=1 and path like '/%';
	update hit_stats  set path=substr(path, 2) where event=1 and path like '/%';

	drop index "hit_counts#site#hour#event";
	create index "hit_counts#site#hour" on hit_counts(site, hour);
	alter table hit_counts drop constraint "hit_counts#site#path#hour#event";
	alter table hit_counts add constraint "hit_counts#site#path#hour" unique(site, path, hour);

	alter table hit_stats       drop column event;
	alter table browser_stats   drop column event;
	alter table system_stats    drop column event;
	alter table location_stats  drop column event;
	alter table ref_stats       drop column event;
	alter table size_stats      drop column event;

	insert into version values ('2020-05-23-1-event');
commit;

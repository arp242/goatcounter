begin;
	update hits       set path=substr(path, 2) where event=1 and path like '/%';
	update hit_counts set path=substr(path, 2) where event=1 and path like '/%';
	update hit_stats  set path=substr(path, 2) where event=1 and path like '/%';

	-- drop index "hit_counts#site#hour#event";
	-- create index "hit_counts#site#hour" on hit_counts(site, hour);
	-- alter table hit_counts drop constraint "hit_counts#site#path#hour#event";
	-- alter table hit_counts add constraint "hit_counts#site#path#hour" unique(site, path, hour);
	create table hit_counts2 (
		site          int        not null check(site>0),
		path          varchar    not null,
		title         varchar    not null,
		event         integer    not null default 0,
		hour          timestamp  not null check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
		total         int        not null,
		total_unique  int        not null,

		constraint "hit_counts2#site#path#hour" unique(site, path, hour) on conflict replace
	);
	insert into hit_counts2 select
		site, path, title, event, hour, total, total_unique
	from hit_counts;
	drop table hit_counts;
	alter table hit_counts2 rename to hit_counts;
	create index "hit_counts#site#hour" on hit_counts(site, hour);

	-- alter table hit_stats       drop column event;
	create table hit_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		path           varchar        not null,
		title          varchar        not null default '',
		stats          varchar        not null,
		stats_unique   varchar        not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into hit_stats2 select
		site, day, path, title, stats, stats_unique
	from hit_stats;
	drop table hit_stats;
	alter table hit_stats2 rename to hit_stats;
	create index "hit_stats#site#day" on hit_stats(site, day);

	-- alter table browser_stats   drop column event;
	create table browser_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		browser        varchar        not null,
		version        varchar        not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into browser_stats2 select
		site, day, browser, version, count, count_unique
	from browser_stats;
	drop table browser_stats;
	alter table browser_stats2 rename to browser_stats;
	create index "browser_stats#site#day"         on browser_stats(site, day);
	create index "browser_stats#site#day#browser" on browser_stats(site, day, browser);

	-- alter table system_stats    drop column event;
	create table system_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		system         varchar        not null,
		version        varchar        not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into system_stats2 select
		site, day, system, version, count, count_unique
	from system_stats;
	drop table system_stats;
	alter table system_stats2 rename to system_stats;
	create index "system_stats#site#day"        on system_stats(site, day);
	create index "system_stats#site#day#system" on system_stats(site, day, system);

	-- alter table location_stats  drop column event;
	create table location_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		location       varchar        not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into location_stats2 select
		site, day, location, count, count_unique
	from location_stats;
	drop table location_stats;
	alter table location_stats2 rename to location_stats;
	create index "location_stats#site#day"          on location_stats(site, day);
	create index "location_stats#site#day#location" on location_stats(site, day, location);

	-- alter table ref_stats       drop column event;
	create table ref_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		ref            varchar        not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into ref_stats2 select
		site, day, ref, count, count_unique
	from ref_stats;
	drop table ref_stats;
	alter table ref_stats2 rename to ref_stats;
	create index "ref_stats#site#day" on ref_stats(site, day);

	-- alter table size_stats      drop column event;
	create table size_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		width          int            not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into size_stats2 select
		site, day, width, count, count_unique
	from size_stats;
	drop table size_stats;
	alter table size_stats2 rename to size_stats;
	create index "size_stats#site#day"       on size_stats(site, day);
	create index "size_stats#site#day#width" on size_stats(site, day, width);


	update hits set path=substr(path, 1) where event=1 and path like '/%';
	insert into version values ('2020-05-23-1-event');
commit;


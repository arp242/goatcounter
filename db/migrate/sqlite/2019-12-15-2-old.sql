begin;
	create temporary table hit_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		path           varchar        not null,
		stats          varchar        not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	insert into hit_stats2 select site, day, path, stats from hit_stats;

	drop table hit_stats;
	alter table hit_stats2 rename to hit_stats;
	create index "hit_stats#site#day" on hit_stats(site, day);

	insert into version values ('2019-12-15-2-old');
commit;

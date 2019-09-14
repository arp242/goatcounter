begin;
	create table browser_stats (
		site           integer        not null                 check(site > 0),

		day            date           not null,
		browser        varchar        not null,
		version        varchar        not null,
		count          int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	create index "browser_stats#site#day"         on browser_stats(site, day);
	create index "browser_stats#site#day#browser" on browser_stats(site, day, browser);

	-- Make sure all stats are re-run
	update sites set last_stat=null;
	delete from hit_stats;

	-- 20 is too much when browsers are at the end.
	update sites set settings = replace(replace(settings, ',"browser":20', ''), '"page":20', '"page":10');

	insert into version values ('2019-08-31-1-browsers');
commit;

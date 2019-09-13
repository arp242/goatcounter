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

	insert into version values ('2019-08-31-1-browsers');
commit;

begin;
	create table browser_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		browser        varchar        not null,
		version        varchar        not null,
		count          int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	insert into browser_stats2 select site, day, browser, version, count from browser_stats;
	drop table browser_stats;
	alter table browser_stats2 rename to browser_stats;

	insert into version values ('2020-01-24-1-rm-mobile');
commit;


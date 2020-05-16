begin;
	create table system_stats (
		site           integer        not null                 check(site > 0),

		day            date           not null,
		event          integer        default 0,
		system         varchar        not null,
		version        varchar        not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	create index "system_stats#site#day"        on system_stats(site, day);
	create index "system_stats#site#day#system" on system_stats(site, day, system);

	insert into version values ('2020-05-16-1-os_stats');
commit;

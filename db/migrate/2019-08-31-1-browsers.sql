begin;
	create table browser_stats (
		site           integer        not null                 check(site > 0),

		day            date           not null,
		browser        varchar        not null,
		count          int            not null,

		created_at     timestamp      null,
		updated_at     timestamp,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	create table platform_stats (
		site           integer        not null                 check(site > 0),

		day            date           not null,
		platform       varchar        not null,
		count          int            not null,

		created_at     timestamp      null,
		updated_at     timestamp,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	insert into version values ('2019-08-31-1-browsers');
commit;

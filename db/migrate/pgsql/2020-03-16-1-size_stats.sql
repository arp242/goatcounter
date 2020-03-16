begin;
	create table size_stats (
		site           integer        not null                 check(site > 0),

		day            date           not null,
		width          int           not null,
		count          int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	create index "size_stats#site#day"       on size_stats(site, day);
	create index "size_stats#site#day#width" on size_stats(site, day, width);

	insert into version values ('2020-03-16-1-size_stats');
commit;

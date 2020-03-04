begin;

	create table ref_stats (
		site           integer        not null                 check(site > 0),

		day            date           not null,
		ref            varchar        not null,
		count          int            not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	create index "ref_stats#site#day" on ref_stats(site, day);

	insert into version values ('2020-02-24-1-ref_stats');
commit;

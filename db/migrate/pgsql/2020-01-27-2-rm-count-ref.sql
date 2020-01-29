begin;
	-- https://www.depesz.com/2016/06/14/incrementing-counters-in-database/
	drop table if exists usage;
	create table usage (
		site           integer        not null                 check(site > 0),
		domain         varchar        not null,
		count          integer        not null,
		vetted         integer        default 0,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	insert into usage
		select site, count_ref, count(count_ref) from hits where count_ref != '' group by site, count_ref;
	alter table hits drop column count_ref;

	insert into version values ('2020-01-27-2-rm-count-ref');
commit;

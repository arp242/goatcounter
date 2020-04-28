begin;
	create table usage (
		site           integer        not null                 check(site > 0),
		domain         varchar        not null,
		count          integer        not null,
		vetted         integer        default 0,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);

	create table hits2 (
		site           integer        not null                 check(site > 0),

		path           varchar        not null,
		ref            varchar        not null,
		ref_original   varchar,
		ref_params     varchar,
		ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o')),
		browser        varchar        not null,
		size           varchar        not null default '',
		location       varchar        not null default '',
		bot            int            default 0,
		title          varchar        not null default '',
		event          int            default 0,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
	);
	insert into hits2 select
		site, path, ref, ref_original, ref_params, ref_scheme, browser, size, location, bot, title, event, created_at
	from hits;
	drop table hits;
	alter table hits2 rename to hits;

	insert into version values ('2020-01-27-2-rm-count-ref');
commit;

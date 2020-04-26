begin;
	-- alter table hits drop constraint hits_ref_scheme_check;
	-- alter table hits add constraint hits_ref_scheme_check check(ref_scheme in ('h', 'g', 'o', 'c'));
	create table hits2 (
		id             integer        primary key autoincrement,
		site           integer        not null                 check(site > 0),
		session        integer        default null,

		path           varchar        not null,
		title          varchar        not null default '',
		event          int            default 0,
		bot            int            default 0,
		ref            varchar        not null,
		ref_original   varchar,
		ref_params     varchar,
		ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
		browser        varchar        not null,
		size           varchar        not null default '',
		location       varchar        not null default '',
		started_session int           default 0,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
	);

	insert into hits2 select * from hits;
	drop table hits;
	rename hits2 to hits;

	insert into version values ('2020-04-22-1-campaigns');
commit;

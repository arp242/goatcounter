begin;
	-- Unused
	drop index "browser_stats#site#day";
	drop index "location_stats#site#day";
	drop index "size_stats#site#day";
	drop index "users#login_request";
	drop index "users#login_token";

	-- Some small performance gains (small tables, but called a lot).
	create index "session_paths#session#path" on session_paths(session, lower(path));
	create index "updates#show_at" on updates(show_at);
	create index "sessions#last_seen" on sessions(last_seen);

	-- Not really used, and don't see it being used in the future.
	-- alter table hits drop column ref_original;
	-- alter table hits drop column ref_params;
	-- alter table hits drop column id;
	create table hits2 (
		id             integer        primary key autoincrement,
		site           integer        not null                 check(site > 0),
		session        integer        default null,

		path           varchar        not null,
		title          varchar        not null default '',
		event          int            default 0,
		bot            int            default 0,
		ref            varchar        not null,
		ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
		browser        varchar        not null,
		size           varchar        not null default '',
		location       varchar        not null default '',
		first_visit    int            default 0,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
	);

	insert into hits2 select
		id, site, session, path, title, event, bot, ref, ref_scheme, browser, size, location, first_visit, created_at
	from hits;
	drop table hits;
	alter table hits2 rename to hits;
	create index "hits#site#bot#created_at"      on hits(site, bot, created_at);
	create index "hits#site#path" on hits(site, lower(path));

	insert into version values ('2020-05-23-1-index');
commit;

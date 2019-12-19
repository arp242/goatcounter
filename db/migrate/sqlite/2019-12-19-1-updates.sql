begin;
	create table updates (
		id             integer        primary key autoincrement,
		subject        varchar        not null,
		body           varchar        not null,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
		show_at        timestamp      not null                 check(show_at = strftime('%Y-%m-%d %H:%M:%S', show_at))
	);

	alter table users add column
		seen_updates_at timestamp     not null default '1970-01-01 00:00:00' check(seen_updates_at = strftime('%Y-%m-%d %H:%M:%S', seen_updates_at));

	insert into version values ('2019-12-19-1-updates');
commit;

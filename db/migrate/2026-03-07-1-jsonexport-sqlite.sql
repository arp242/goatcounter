create table exports2 (
	export_id      integer primary key autoincrement,
	site_id        integer        not null,
	format         varchar        not null,
	start_from_hit_id integer,
	start_from_day timestamp                  check(start_from_day = strftime('%Y-%m-%d %H:%M:%S', start_from_day)),

	path           varchar        not null,
	created_at     timestamp      not null    check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

	finished_at    timestamp                  check(finished_at is null or finished_at = strftime('%Y-%m-%d %H:%M:%S', finished_at)),
	last_hit_id    integer,
	num_rows       integer,
	size           varchar,
	hash           varchar,
	error          varchar
);
insert into exports2 (
		export_id, site_id, format, start_from_hit_id, start_from_day, path,
		created_at, finished_at, last_hit_id, num_rows, size, hash, error
	)
	select
		export_id, site_id, 'csv', start_from_hit_id, null, path, created_at,
		finished_at, last_hit_id, num_rows, size, hash, error
	from exports
;

drop table exports;
alter table exports2 rename to exports;
create index "exports#site_id#created_at" on exports(site_id, created_at);

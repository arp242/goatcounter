begin;
	create table exports (
		export_id         integer        primary key autoincrement,
		site_id           integer        not null,
		start_from_hit_id integer        not null,

		path              varchar        not null,
		created_at        timestamp      not null    check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

		finished_at       timestamp                  check(finished_at is null or finished_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
		last_hit_id       integer,
		num_rows          integer,
		size              varchar,
		hash              varchar,
		error             varchar,

		foreign key (site_id) references sites(id) on delete restrict on update restrict
	);
	create index "exports#site_id#created_at" on exports(site_id, created_at);

	insert into version values('2020-06-26-1-record-export');
commit;

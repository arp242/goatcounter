begin;
	create table exports (
		export_id         serial         primary key,
		site_id           integer        not null,
		start_from_hit_id integer        not null,

		path              varchar        not null,
		created_at        timestamp      not null,

		finished_at       timestamp,
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

begin;
	create table updates (
		id             serial         primary key,
		subject        varchar        not null,
		body           varchar        not null,

		created_at     timestamp      not null,
		show_at        timestamp      not null
	);

	alter table users add column
		seen_updates_at timestamp     not null default current_timestamp;

	insert into version values ('2019-12-19-1-updates');
commit;

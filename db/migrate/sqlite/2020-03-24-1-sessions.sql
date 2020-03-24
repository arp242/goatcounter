begin;
	create table sessions (
		id             serial         primary key,
		site           integer        not null                 check(site > 0),
		hash           bytea          null,
		created_at     timestamp      not null,
		last_seen      timestamp      not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	create unique index "sessions#site#hash" on sessions(site, hash);

	alter table hits add column session int default null;
	alter table hits add column started_session int default 0;

	alter table hit_stats      add column stats_unique varchar not null default '';
	alter table browser_stats  add column count_unique int not null default 0;
	alter table location_stats add column count_unique int not null default 0;
	alter table ref_stats      add column count_unique int not null default 0;
	alter table size_stats     add column count_unique int not null default 0;

	-- alter table hit_stats      alter column stats_unique drop default;
	-- alter table browser_stats  alter column count_unique drop default;
	-- alter table location_stats alter column count_unique drop default;
	-- alter table ref_stats      alter column count_unique drop default;
	-- alter table size_stats     alter column count_unique drop default;

	insert into version values ('2020-03-24-1-sessions');
commit;

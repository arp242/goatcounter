begin;
	update sites set settings = json_set(settings, '$.collect', json('30'));

	-- alter table hits alter column user_agent_id drop not null;
	-- alter table hits drop constraint hits_user_agent_id_check;
	-- alter table hits add constraint hits_user_agent_id_check check(user_agent_id != 0);

	create table hits2 (
		hit_id         integer        primary key autoincrement,
		site_id        integer        not null                 check(site_id > 0),
		path_id        integer        not null                 check(path_id > 0),
		user_agent_id  integer        null                     check(user_agent_id != 0),

		session        blob           default null,
		bot            integer        default 0,
		ref            varchar        not null,
		ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
		size           varchar        not null default '',
		location       varchar        not null default '',
		first_visit    integer        default 0,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
	);
	insert into hits2 (hit_id, site_id, path_id, user_agent_id, session, bot, ref, ref_scheme, size, location, first_visit, created_at)
		select hit_id, site_id, path_id, user_agent_id, session, bot, ref, ref_scheme, size, location, first_visit, created_at from hits;
	drop table hits;
	alter table hits2 rename to hits;
	create index "hits#site_id#created_at" on hits(site_id, created_at );

	insert into version values('2020-12-24-1-user_agent_id_null');
commit;

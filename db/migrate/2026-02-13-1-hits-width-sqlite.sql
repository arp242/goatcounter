create table hits_new (
	hit_id         integer primary key autoincrement,
	site_id        integer        not null,
	path_id        integer        not null,
	ref_id         integer        not null default 1,

	session        blob           default null,
	first_visit    integer        default 0,

	browser_id     integer        not null,
	system_id      integer        not null,
	campaign       integer        default null,
	width          int            null,
	location       varchar        not null default '',
	language       varchar,

	created_at     timestamp      not null              check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);

insert into hits_new (hit_id, site_id, path_id, ref_id, session, first_visit, browser_id, system_id, campaign, width, location, language, created_at)
	select hit_id, site_id, path_id, ref_id, session, first_visit, browser_id, system_id, campaign, width, location, language, created_at
	from hits;

drop table hits;

alter table hits_new rename to hits;
create index "hits#site_id#created_at" on hits(site_id, created_at desc);

begin;
	-- Create new paths table
	create table paths (
		path_id        integer        primary key autoincrement,
		site           integer        not null                 check(site > 0),

		path           varchar        not null,
		title          varchar        not null default '',
		event          int            default 0
	);

	-- Copy over the data.
	insert into paths (site, path, title, event)
		select
			site,
			path,
			min(title), -- TODO: ideally this should be "latest title".
			max(event)
		from hits
		group by site, path;
	create unique index "paths#site#path"  on paths(site, path); -- TODO: should be lower()
	create index        "paths#path#title" on paths(lower(path), lower(title));

	-- Create new user_agents table
	create table user_agents (
			user_agent_id    integer        primary key autoincrement,
			ua               varchar        not null,
			bot              int            not null,
			browser          varchar,
			browser_version  varchar,
			system           varchar,
			system_version   varchar
	);

	-- Copy over the data.
	insert into user_agents (ua, bot) select browser, max(bot) from hits group by browser;
	update user_agents set bot=0 where bot not in (0, 3, 4, 5, 6, 7); -- Others are "not a bot" or because of IP ranges.
	create unique index "user_agents#ua" on user_agents(ua);

	-- Add new columns.
	alter table hits       add column path_id int not null default 0;
	alter table hits       add column user_agent_id int not null default 0;

	-- TODO: not null?
	alter table hit_stats  add column path_id int default 0;
	alter table hit_counts add column path_id int default 0;
	alter table ref_counts add column path_id int default 0;

	-- Update hits.
	create index tmp on hits(browser);
	update hits set
		path_id=(select path_id from paths where paths.site=hits.site and paths.path=hits.path),
		user_agent_id=(select user_agent_id from user_agents where ua=hits.browser);
	drop index tmp;

	-- Update other tables.
	update hit_stats  set path_id=(select path_id from paths where paths.site=hit_stats.site and paths.path=hit_stats.path);
	update hit_counts set path_id=(select path_id from paths where paths.site=hit_counts.site and paths.path=hit_counts.path);
	update ref_counts set path_id=(select path_id from paths where paths.site=ref_counts.site and paths.path=ref_counts.path);

	-- Make some new indexes.
	-- create index "hit_stats#path_id#day" on hit_stats(path_id, day);
	-- create index "hits#path_id#bot#created_at" on hits(path_id, bot, created_at);
	-- create index "hit_counts#path_id" on hit_counts(path_id);

	-- Remove old columns.
	-- alter table hits drop column path;
	-- alter table hits drop column title;
	-- alter table hits drop column event;
	-- alter table hits drop column browser;
	create table hits2 (
		id             integer        primary key autoincrement,
		site           integer        not null                 check(site > 0),
		session        integer        default null,
		path_id        int            not null                 check(path_id > 0),
		user_agent_id  int            not null                 check(user_agent_id > 0),

		bot            int            default 0,
		ref            varchar        not null,
		ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
		size           varchar        not null default '',
		location       varchar        not null default '',
		first_visit    int            default 0,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
	);
	insert into hits2 select
		id, site, session, path_id, user_agent_id, bot, ref, ref_scheme, size, location, first_visit, created_at
	from hits;
	drop table hits;
	alter table hits2 rename to hits;
	create index "hits#site#bot#created_at"    on hits(site, bot, created_at);
	create index "hits#path_id#bot#created_at" on hits(path_id, bot, created_at);

	-- alter table hit_stats  drop column path;
	-- alter table hit_stats  drop column title;
	-- alter table hit_stats  drop column event;
	create table hit_stats2 (
		site           integer        not null                 check(site > 0),
		path_id        int            not null                 check(path_id > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		stats          varchar        not null,
		stats_unique   varchar        not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into hit_stats2 select
		site, path_id, day, stats, stats_unique
	from hit_stats;
	drop table hit_stats;
	alter table hit_stats2 rename to hit_stats;
	create index "hit_stats#site#day" on hit_stats(site, day);
	create index "hit_stats#path_id#day" on hit_stats(path_id, day);

	-- alter table hit_counts drop column path;
	-- alter table hit_counts drop column title;
	-- alter table hit_counts drop column event;
	create table hit_counts2 (
		site          int        not null check(site>0),
		path_id       int        not null check(path_id > 0),

		hour          timestamp  not null check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
		total         int        not null,
		total_unique  int        not null,

		constraint "hit_counts#site#path_id#hour" unique(site, path_id, hour) on conflict replace
	);
	insert into hit_counts2 select
		site, path_id, hour, total, total_unique
	from hit_counts;
	drop table hit_counts;
	alter table hit_counts2 rename to hit_counts;
	create index "hit_counts#site#hour" on hit_counts(site, hour);
	create index "hit_counts#path_id" on hit_counts(path_id);

	-- alter table ref_counts drop column path;
	create table ref_counts2 (
		site          int        not null check(site>0),
		path_id       int        not null check(path_id > 0),

		ref           varchar    not null,
		ref_scheme    varchar    null,
		hour          timestamp  not null check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
		total         int        not null,
		total_unique  int        not null,

		constraint "ref_counts#site#path_id#ref#hour" unique(site, path_id, ref, hour) on conflict replace
	);
	insert into ref_counts2 select
		site, path_id, ref, ref_scheme, hour, total, total_unique
	from ref_counts;
	drop table ref_counts;
	alter table ref_counts2 rename to ref_counts;
	create index "ref_counts#site#hour" on ref_counts(site, hour);
	create index "ref_counts#path_id"   on ref_counts(path_id);

	insert into version values('2020-06-08-1-paths');
commit;

begin;
	-- TODO: fk

	-- Create new paths table
	create table paths (
		path_id        serial         primary key,
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
			user_agent_id    serial         primary key,
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
	alter table hits       add column path_id       int not null default 0;
	alter table hits       add column user_agent_id int not null default 0;

	-- TODO: not null causes problems?
	alter table hit_stats  add column path_id int not null default 0;
	alter table hit_counts add column path_id int not null default 0;
	alter table ref_counts add column path_id int not null default 0;

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
	create index "hit_stats#path_id#day" on hit_stats(path_id, day);
	create index "hits#path_id#bot#created_at" on hits(path_id, bot, created_at);
	create index "hit_counts#path_id" on hit_counts(path_id);
	create index "ref_counts#path_id" on ref_counts(path_id);

	-- Remove old columns.
	alter table hits drop column path;
	alter table hits drop column title;
	alter table hits drop column event;
	alter table hits drop column browser;
	alter table hit_stats  drop column path;
	alter table hit_stats  drop column title;
	alter table hit_stats  drop column event;
	alter table hit_counts drop column path;
	alter table hit_counts drop column title;
	alter table hit_counts drop column event;
	alter table ref_counts drop column path;

	-- Vacuum and write WAL
	-- checkpoint; vacuum full; checkpoint;

	insert into version values('2020-06-08-1-paths');
commit;

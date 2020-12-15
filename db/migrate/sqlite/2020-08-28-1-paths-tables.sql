begin;
	----------------
	-- NEW TABLES --
	----------------
	create table paths (
		path_id        integer        primary key autoincrement,
		site_id        integer        not null,
		path           varchar        not null,
		title          varchar        not null default '',
		event          int            default 0,

		foreign key (site_id) references sites(id) on delete restrict on update restrict
	);

	create table user_agents (
		user_agent_id    integer        primary key autoincrement,
		browser_id       integer        not null,
		system_id        integer        not null,

		ua               varchar        not null,
		bot              integer        not null
	);

	create table systems (
		system_id        integer        primary key autoincrement,
		name             varchar,
		version          varchar
	);

	create table browsers (
		browser_id       integer        primary key autoincrement,
		name             varchar,
		version          varchar
	);

	------------------------
	-- Copy over the data --
	------------------------
	insert into paths (site_id, path, title, event)
		select
			site,
			min(path),
            max(title),
			max(event)
		from hits
		group by site, lower(path);
	create unique index "paths#site_id#path" on paths(site_id, lower(path));
	create        index "paths#path#title"   on paths(lower(path), lower(title));

	insert into user_agents (ua, bot, browser_id, system_id)
		select browser, max(bot), 0, 0 from hits group by browser;
	update user_agents set bot=0 where bot not in (0, 3, 4, 5, 6, 7); -- Others are "not a bot" or because of IP ranges.
	create unique index "user_agents#ua" on user_agents(ua);

	-- Truncate the data; will be re-created on reindex later.
	delete from browser_stats;
	delete from hit_counts;
	delete from hit_stats;
	delete from location_stats;
	delete from ref_counts;
	delete from system_stats;
	delete from size_stats;

	---------------------
	-- Add new columns --
	---------------------
	alter table hits          add column path_id       int default 0 not null;
	alter table hits          add column user_agent_id int default 0 not null;
	alter table hit_stats     add column path_id       int not null;
	alter table hit_counts    add column path_id       int not null;
	alter table ref_counts    add column path_id       int not null;
	alter table browser_stats add column browser_id    int not null;
	alter table system_stats  add column system_id     int not null;

	insert into version values('2020-08-28-1-paths-tables');
commit;

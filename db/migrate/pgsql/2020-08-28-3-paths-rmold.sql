begin;
	-----------------------
	-- Rename id columns --
	-----------------------
	alter table sites rename id to site_id;

	alter table users rename id   to user_id;
	alter table users rename site to site_id;

	alter table hits rename id       to hit_id;
	alter table hits rename site     to site_id;

	alter table hit_stats      rename site to site_id;
	alter table hit_counts     rename site to site_id;
	alter table ref_counts     rename site to site_id;
	alter table browser_stats  rename site to site_id;
	alter table system_stats   rename site to site_id;
	alter table location_stats rename site to site_id;
	alter table size_stats     rename site to site_id;

	alter table hits drop column session;
	alter table hits rename session2 to session;

	--------------------------------------
	-- Make new indexes and constraints --
	--------------------------------------
	create index "sites#parent" on sites(parent) where state='a';

	drop index "hits#site#bot#created_at";
	create index "hits#site_id#created_at" on hits(site_id, created_at);
	cluster hits using "hits#site_id#created_at";

	-- hit_counts
	alter table hit_counts add constraint "hit_counts#site_id#path_id#hour" unique(site_id, path_id, hour);
	cluster hit_counts using "hit_counts#site_id#path_id#hour";
	create index "hit_counts#path_id" on hit_counts(path_id);
	alter table hit_counts replica identity using index "hit_counts#site_id#path_id#hour";
	drop index "hit_counts#site#hour";

	-- ref_counts
	alter table ref_counts add constraint "ref_counts#site_id#path_id#ref#hour" unique(site_id, path_id, ref, hour);
	cluster ref_counts using "ref_counts#site_id#path_id#ref#hour";
	create index "ref_counts#path_id" on ref_counts(path_id);
	alter table ref_counts replica identity using index "ref_counts#site_id#path_id#ref#hour";
	drop index "ref_counts#site#hour";

	-- hit_stats
	create unique index "hit_stats#site_id#path_id#day" on hit_stats(site_id, path_id, day);
	cluster hit_stats using "hit_stats#site_id#path_id#day";
	alter table hit_stats replica identity using index "hit_stats#site_id#path_id#day";
	drop index "hit_stats#site#day";

	-- browser_stats
	create unique index "browser_stats#site_id#day#browser_id" on browser_stats(site_id, day, browser_id);
	cluster browser_stats using "browser_stats#site_id#day#browser_id";
	alter table browser_stats replica identity using index "browser_stats#site_id#day#browser_id";

	-- system_stats
	create unique index "system_stats#site_id#day#system_id" on system_stats(site_id, day, system_id);
	cluster system_stats using "system_stats#site_id#day#system_id";
	alter table system_stats replica identity using index "system_stats#site_id#day#system_id";
	drop index "system_stats#site#day";

	-- location_stats
	cluster location_stats using "location_stats#site#day#location";

	-- size_stats
	cluster size_stats using "size_stats#site#day#width";

	------------------------
	-- Remove old columns --
	------------------------
	alter table hits drop column path;
	alter table hits drop column title;
	alter table hits drop column event;
	alter table hits drop column browser;

	alter table hit_stats  drop column path;
	alter table hit_stats  drop column title;

	alter table hit_counts drop column path;
	alter table hit_counts drop column title;
	alter table hit_counts drop column event;

	alter table ref_counts drop column path;

	alter table browser_stats drop column browser;
	alter table browser_stats drop column version;

	alter table system_stats  drop column system;
	alter table system_stats  drop column version;

	insert into version values('2020-08-28-3-paths-rmold');
commit;

begin;
	------------------------
	-- Rename/add columns --
	------------------------
	alter table sites rename id to site_id;
	alter table sites add column first_hit_at timestamp;
	update sites set first_hit_at=created_at;
	alter table sites alter column first_hit_at set not null;
	alter table sites drop constraint sites_parent_check;
	alter sequence sites_id_seq rename to sites_site_id_seq;

	alter table users rename id   to user_id;
	alter table users rename site to site_id;
	alter table users drop constraint users_site_check;
	alter sequence users_id_seq rename to users_user_id_seq;
	alter index "users#site" rename to "users#site_id";
	alter index "users#site#email" rename to "users#site_id#email";
	alter table users rename constraint users_site_fkey to users_site_id_fkey;

	alter table hits rename id       to hit_id;
	alter table hits rename site     to site_id;
	alter table hits add check(path_id > 0);
	alter table hits add check(user_agent_id > 0);
	alter table hits drop column session;
	alter table hits rename session2 to session;
	alter table hits rename constraint hits_site_check to hits_site_id_check;
	alter sequence hits_id_seq rename to hits_hit_id_seq;

	alter table hit_counts rename site to site_id;
	alter table hit_counts add foreign key (site_id) references sites(site_id) on delete restrict on update restrict;
	alter table hit_counts drop constraint hit_counts_site_check;

	alter table ref_counts rename site to site_id;
	alter table ref_counts add foreign key (site_id) references sites(site_id) on delete restrict on update restrict;
	alter table ref_counts drop constraint ref_counts_site_check;

	alter table hit_stats  rename site to site_id;
	alter table hit_stats drop constraint hit_stats_site_check;
	alter table hit_stats rename constraint hit_stats_site_fkey to hit_stats_site_id_fkey;

	alter table browser_stats rename site to site_id;
	alter table browser_stats add column path_id int not null;
	alter table browser_stats drop constraint browser_stats_site_check;
	alter table browser_stats add foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict;
	alter table browser_stats rename constraint browser_stats_site_fkey to browser_stats_site_id_fkey;

	alter table system_stats rename site to site_id;
	alter table system_stats drop constraint system_stats_site_check;
	alter table system_stats add column path_id int not null;
	alter table system_stats add foreign key (system_id) references systems(system_id) on delete restrict on update restrict;
	alter table system_stats rename constraint system_stats_site_fkey to system_stats_site_id_fkey;
	drop index "system_stats#site#day";

	alter table location_stats rename site to site_id;
	alter table location_stats drop constraint location_stats_site_check;
	alter table location_stats add column path_id int not null;
	alter table location_stats rename constraint location_stats_site_fkey to location_stats_site_id_fkey;

	alter table size_stats rename site to site_id;
	alter table size_stats drop constraint size_stats_site_check;
	alter table size_stats add column path_id int not null;
	alter table size_stats rename constraint size_stats_site_fkey to size_stats_site_id_fkey;


	--------------------------------------
	-- Make new indexes and constraints --
	--------------------------------------
	create index "sites#parent" on sites(parent) where state='a';

	drop index "hits#site#bot#created_at";
	create index "hits#site_id#created_at" on hits(site_id, created_at desc);
	cluster hits using "hits#site_id#created_at";

	-- hit_counts
	alter table hit_counts add constraint "hit_counts#site_id#path_id#hour" unique(site_id, path_id, hour);
	alter table hit_counts replica identity using index "hit_counts#site_id#path_id#hour";
	drop index "hit_counts#site#hour";
	create index "hit_counts#site_id#hour" on hit_counts(site_id, hour desc);
	cluster hit_counts using "hit_counts#site_id#hour";


	-- ref_counts
	alter table ref_counts add constraint "ref_counts#site_id#path_id#ref#hour" unique(site_id, path_id, ref, hour);
	alter table ref_counts replica identity using index "ref_counts#site_id#path_id#ref#hour";
	drop index "ref_counts#site#hour";
	create index "ref_counts#site_id#hour" on ref_counts(site_id, hour desc);
	cluster ref_counts using "ref_counts#site_id#hour";

	-- hit_stats
	drop index "hit_stats#site#day";
	create index "hit_stats#site_id#day" on hit_stats(site_id, day desc);

	-- browser_stats
	create index "browser_stats#site_id#browser_id#day" on browser_stats(site_id, browser_id, day desc);

	-- system_stats
	create index "system_stats#site_id#system_id#day" on system_stats(site_id, system_id, day desc);

	-- location_stats
	drop index "location_stats#site#day#location";
    create index "location_stats#site_id#day" on location_stats(site_id, day desc);

	-- size_stats
	drop index "size_stats#site#day#width";
    create index "size_stats#site_id#day" on size_stats(site_id, day desc);

	-- store
	alter table store replica identity using index "store#key";

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

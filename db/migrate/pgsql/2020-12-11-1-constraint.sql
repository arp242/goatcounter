begin;
	drop index "browser_stats#site_id#path_id#day#browser_id";
	drop index "hit_stats#site_id#path_id#day";
	drop index "location_stats#site_id#path_id#day#location";
	drop index "size_stats#site_id#path_id#day#width";
	drop index "system_stats#site_id#path_id#day#system_id";

	alter table size_stats     add constraint "size_stats#site_id#path_id#day#width"         unique(site_id, path_id, day, width);
	alter table browser_stats  add constraint "browser_stats#site_id#path_id#day#browser_id" unique(site_id, path_id, day, browser_id);
	alter table hit_stats      add constraint "hit_stats#site_id#path_id#day"                unique(site_id, path_id, day);
	alter table location_stats add constraint "location_stats#site_id#path_id#day#location"  unique(site_id, path_id, day, location);
	alter table system_stats   add constraint "system_stats#site_id#path_id#day#system_id"   unique(site_id, path_id, day, system_id);

	alter table size_stats     replica identity using index "size_stats#site_id#path_id#day#width";
	alter table browser_stats  replica identity using index "browser_stats#site_id#path_id#day#browser_id";
	alter table hit_stats      replica identity using index "hit_stats#site_id#path_id#day";
	alter table location_stats replica identity using index "location_stats#site_id#path_id#day#location";
	alter table system_stats   replica identity using index "system_stats#site_id#path_id#day#system_id";

	insert into version values('2020-12-11-1-constraint');
commit;

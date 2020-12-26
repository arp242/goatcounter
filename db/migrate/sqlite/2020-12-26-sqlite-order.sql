begin;
	drop index "hits#site_id#created_at";
	drop index "hit_counts#site_id#hour";
	drop index "ref_counts#site_id#hour";
	drop index "hit_stats#site_id#day";
	drop index "browser_stats#site_id#browser_id#day";
	drop index "system_stats#site_id#system_id#day";
	drop index "location_stats#site_id#day";
	drop index "size_stats#site_id#day";

	create index "hits#site_id#created_at" on hits(site_id, created_at desc);
	create index "hit_counts#site_id#hour" on hit_counts(site_id, hour desc);
	create index "ref_counts#site_id#hour" on ref_counts(site_id, hour desc);
	create index "hit_stats#site_id#day" on hit_stats(site_id, day desc);
	create index "browser_stats#site_id#browser_id#day" on browser_stats(site_id, browser_id, day desc);
	create index "system_stats#site_id#system_id#day" on system_stats(site_id, system_id, day desc);
	create index "location_stats#site_id#day" on location_stats(site_id, day desc);
	create index "size_stats#site_id#day" on size_stats(site_id, day desc);

	insert into version values('2020-12-26-sqlite-order');
commit;

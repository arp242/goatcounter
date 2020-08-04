begin;
    drop index "size_stats#site#day#width";
    create unique index "size_stats#site#day#width" on size_stats(site, day, width);
    drop index "location_stats#site#day#location";
    create unique index "location_stats#site#day#location" on location_stats(site, day, location);

    alter table store alter column key set not null;

    alter table location_stats  replica identity using index "location_stats#site#day#location";
    alter table size_stats      replica identity using index "size_stats#site#day#width";
    alter table hit_counts      replica identity using index "hit_counts#site#path#hour";
    alter table ref_counts      replica identity using index "ref_counts#site#path#ref#hour";
    alter table store           replica identity using index "store#key";

    alter table hit_stats       replica identity full;
    alter table browser_stats   replica identity full;
    alter table system_stats    replica identity full;

	insert into version values('2020-08-01-1-repl');
commit;

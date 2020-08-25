begin;
    drop index "size_stats#site#day#width";
    create unique index "size_stats#site#day#width" on size_stats(site, day, width);
    drop index "location_stats#site#day#location";
    create unique index "location_stats#site#day#location" on location_stats(site, day, location);

    -- alter table store alter column key set not null;
	create table store2 (
		key     varchar not null,
		value   text
	);
	insert into store2 select key, value from store;
	drop table store;
	alter table store2 rename to store;
	create unique index "store#key" on store(key);

	drop index if exists "hits#site#bot#path#created_at";
	create index if not exists "hits#site#path" on hits(site, lower(path));

	insert into version values('2020-08-01-1-repl');
commit;

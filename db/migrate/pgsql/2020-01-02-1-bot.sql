begin;
	alter table hits add column bot int default 0;
	drop index "hits#site#created_at";
	drop index "hits#site#path#created_at";
	create index "hits#site#bot#created_at"      on hits(site, bot, created_at);
	create index "hits#site#bot#path#created_at" on hits(site, bot, lower(path), created_at);
	insert into version values ('2020-01-02-1-bot');
commit;

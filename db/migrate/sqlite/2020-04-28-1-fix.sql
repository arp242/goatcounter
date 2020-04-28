-- Turns out SQLite doesn't preserve/rebuild the indexes on migration >_<
begin;
	create index if not exists "browser_stats#site#day"         on browser_stats(site, day);
	create index if not exists "browser_stats#site#day#browser" on browser_stats(site, day, browser);

	create index if not exists "hits#site#bot#created_at"      on hits(site, bot, created_at);
	create index if not exists "hits#site#bot#path#created_at" on hits(site, bot, lower(path), created_at);

	create unique index if not exists "sites#code" on sites(lower(code));

	create index if not exists "hit_stats#site#day" on hit_stats(site, day);

	create unique index if not exists "users#login_request" on users(login_request);
	create unique index if not exists "users#login_token"   on users(login_token);
	create        index if not exists "users#site"          on users(site);
	create unique index if not exists "users#site#email"    on users(site, lower(email));

	insert into version values ('2020-04-28-1-fix');
commit;

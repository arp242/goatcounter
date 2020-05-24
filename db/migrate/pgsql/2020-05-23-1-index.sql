begin;
	-- Unused
	drop index if exists "browser_stats#site#day";
	drop index if exists "location_stats#site#day";
	drop index if exists "size_stats#site#day";
	drop index if exists "users#login_request";
	drop index if exists "users#login_token";

	-- Used for purge
	drop   index if exists     "hits#site#bot#path#created_at";
	create index if not exists "hits#site#path" on hits(site, lower(path));

	-- Some small performance gains
	create index if not exists "session_paths#session#path" on session_paths(session, lower(path));
	create index if not exists "updates#show_at" on updates(show_at);
	create index if not exists "sessions#last_seen" on sessions(last_seen);

	-- Not really used, and don't see it being used in the future.
	alter table hits drop column if exists ref_original;
	alter table hits drop column if exists ref_params;

	insert into version values ('2020-05-23-1-index');
commit;

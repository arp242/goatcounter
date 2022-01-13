-- alter table users          drop constraint users_site_id_fkey;
create table users2 (
	user_id        integer primary key autoincrement,
	site_id        integer        not null,

	email          varchar        not null,
	email_verified integer        not null default 0,
	password       blob       default null,
	totp_enabled   integer        not null default 0,
	totp_secret    blob,
	access         varchar      not null default '{"all":"a"}',
	login_at       timestamp      null,
	login_request  varchar        null,
	login_token    varchar        null,
	csrf_token     varchar        null,
	email_token    varchar        null,
	seen_updates_at timestamp     not null default current_timestamp,
	reset_at       timestamp      null,
	settings       varchar      not null default '{}',
	last_report_at timestamp      not null default current_timestamp,

	created_at     timestamp      not null,
	updated_at     timestamp
);

-- alter table api_tokens     drop constraint api_tokens_site_id_fkey;
-- alter table api_tokens     drop constraint api_tokens_user_id_fkey;
create table api_tokens2 (
	api_token_id   integer primary key autoincrement,
	site_id        integer        not null,
	user_id        integer        not null,

	name           varchar        not null,
	token          varchar        not null                 check(length(token) > 10),
	permissions    varchar      not null,
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);

-- alter table paths          drop constraint paths_site_id_fkey;
create table paths2 (
	path_id        integer primary key autoincrement,
	site_id        integer        not null,

	path           varchar        not null,
	title          varchar        not null default '',
	event          integer        default 0
);

-- alter table exports        drop constraint exports_site_id_fkey;
create table exports2 (
	export_id      integer primary key autoincrement,
	site_id        integer        not null,
	start_from_hit_id integer     not null,

	path           varchar        not null,
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

	finished_at    timestamp                               check(finished_at is null or finished_at = strftime('%Y-%m-%d %H:%M:%S', finished_at)),
	last_hit_id    integer,
	num_rows       integer,
	size           varchar,
	hash           varchar,
	error          varchar
);

-- alter table user_agents    drop constraint user_agents_browser_id_fkey;
-- alter table user_agents    drop constraint user_agents_system_id_fkey;
create table user_agents2 (
	user_agent_id  integer primary key autoincrement,
	browser_id     integer        not null,
	system_id      integer        not null,

	ua             varchar        not null,
	isbot          integer        not null
);

-- alter table hit_counts     drop constraint hit_counts_site_id_fkey;
create table hit_counts2 (
	site_id        integer        not null,
	path_id        integer        not null,

	hour           timestamp      not null                 check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
	total          integer        not null,
	total_unique   integer        not null,

	constraint "hit_counts#site_id#path_id#hour" unique(site_id, path_id, hour) on conflict replace
);

-- alter table ref_counts     drop constraint ref_counts_site_id_fkey;
create table ref_counts2 (
	site_id        integer        not null,
	path_id        integer        not null,

	ref            varchar        not null,
	ref_scheme     varchar        null,
	hour           timestamp      not null                 check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
	total          integer        not null,
	total_unique   integer        not null,

	constraint "ref_counts#site_id#path_id#ref#hour" unique(site_id, path_id, ref, hour) on conflict replace
);

-- alter table hit_stats      drop constraint hit_stats_site_id_fkey;
create table hit_stats2 (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	stats          varchar        not null,
	stats_unique   varchar        not null,

	constraint "hit_stats#site_id#path_id#day" unique(site_id, path_id, day) on conflict replace
);

-- alter table browser_stats  drop constraint browser_stats_site_id_fkey;
-- alter table browser_stats  drop constraint browser_stats_browser_id_fkey;
create table browser_stats2 (
	site_id        integer        not null,
	path_id        integer        not null,
	browser_id     integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	count          integer        not null,
	count_unique   integer        not null,

	constraint "browser_stats#site_id#path_id#day#browser_id" unique(site_id, path_id, day, browser_id) on conflict replace
);

-- alter table system_stats   drop constraint system_stats_site_id_fkey;
-- alter table system_stats   drop constraint system_stats_system_id_fkey;
create table system_stats2 (
	site_id        integer        not null,
	path_id        integer        not null,
	system_id      integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	count          integer        not null,
	count_unique   integer        not null,

	constraint "system_stats#site_id#path_id#day#system_id" unique(site_id, path_id, day, system_id) on conflict replace
);

-- alter table location_stats drop constraint location_stats_site_id_fkey;
create table location_stats2 (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	location       varchar        not null,
	count          integer        not null,
	count_unique   integer        not null,

	constraint "location_stats#site_id#path_id#day#location" unique(site_id, path_id, day, location) on conflict replace
);

-- alter table size_stats     drop constraint size_stats_site_id_fkey;
create table size_stats2 (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	width          integer        not null,
	count          integer        not null,
	count_unique   integer        not null,

	constraint "size_stats#site_id#path_id#day#width" unique(site_id, path_id, day, width) on conflict replace
);

-- alter table language_stats drop constraint language_stats_site_id_fkey;
create table language_stats2 (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	language       varchar        not null,
	count          integer        not null,
	count_unique   integer        not null,

	constraint "language_stats#site_id#path_id#day#language" unique(site_id, path_id, day, language) on conflict replace
);

insert into users2 select
	user_id, site_id, email, email_verified, password, totp_enabled, totp_secret, access, login_at,
	login_request, login_token, csrf_token, email_token, seen_updates_at, reset_at, settings,
	last_report_at, created_at, updated_at
from users;

insert into api_tokens2 select
	api_token_id, site_id, user_id, name, token, permissions, created_at
from api_tokens;

insert into paths2 select
	path_id, site_id, path, title, event
from paths;

insert into exports2 select
	export_id, site_id, start_from_hit_id, path, created_at, finished_at,
	last_hit_id, num_rows, size, hash, error
from exports;

insert into user_agents2 select
	user_agent_id, browser_id, system_id, ua, isbot
from user_agents;

insert into hit_counts2 select
	site_id, path_id, hour, total, total_unique
from hit_counts;

insert into ref_counts2 select
	site_id, path_id, ref, ref_scheme, hour, total, total_unique
from ref_counts;

insert into hit_stats2 select
	site_id, path_id, day, stats, stats_unique
from hit_stats;

insert into browser_stats2 select
	site_id, path_id, browser_id, day, count, count_unique
from browser_stats;

insert into system_stats2 select
	site_id, path_id, system_id, day, count, count_unique
from system_stats;

insert into location_stats2 select
	site_id, path_id, day, location, count, count_unique
from location_stats;

insert into size_stats2 select
	site_id, path_id, day, width, count, count_unique
from size_stats;

insert into language_stats2 select
	site_id, path_id, day, language, count, count_unique
from language_stats;

drop table users;
drop table api_tokens;
drop table paths;
drop table exports;
drop table user_agents;
drop table hit_counts;
drop table ref_counts;
drop table hit_stats;
drop table browser_stats;
drop table system_stats;
drop table location_stats;
drop table size_stats;
drop table language_stats;
alter table users2          rename to users;
alter table api_tokens2     rename to api_tokens;
alter table paths2          rename to paths;
alter table exports2        rename to exports;
alter table user_agents2    rename to user_agents;
alter table hit_counts2     rename to hit_counts;
alter table ref_counts2     rename to ref_counts;
alter table hit_stats2      rename to hit_stats;
alter table browser_stats2  rename to browser_stats;
alter table system_stats2   rename to system_stats;
alter table location_stats2 rename to location_stats;
alter table size_stats2     rename to size_stats;
alter table language_stats2 rename to language_stats;

create index "hit_counts#site_id#hour" on hit_counts(site_id, hour desc);
create index "ref_counts#site_id#hour" on ref_counts(site_id, hour desc);
create index "hit_stats#site_id#day" on hit_stats(site_id, day desc);
create index "browser_stats#site_id#browser_id#day" on browser_stats(site_id, browser_id, day desc);
create index "system_stats#site_id#system_id#day" on system_stats(site_id, system_id, day desc);
create index "location_stats#site_id#day" on location_stats(site_id, day desc);
create index "size_stats#site_id#day" on size_stats(site_id, day desc);
create index "exports#site_id#created_at" on exports(site_id, created_at);
create index "language_stats#site_id#day" on language_stats(site_id, day desc);
create unique index "paths#site_id#path" on paths(site_id, lower(path));
create        index "paths#path#title"   on paths(lower(path), lower(title));
create unique index "api_tokens#site_id#token" on api_tokens(site_id, token);
create        index "users#site_id"       on users(site_id);
create unique index "users#site_id#email" on users(site_id, lower(email));
create unique index "user_agents#ua" on user_agents(ua);

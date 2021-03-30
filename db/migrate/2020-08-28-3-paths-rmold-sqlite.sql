-- alter table sites rename id to site_id;
-- alter table sites add column first_hit_at timestamp;
-- alter table sites drop constraint sites_parent_check;
create table sites2 (
	site_id        integer        primary key autoincrement,
	parent         integer        null,

	code           varchar        not null                 check(length(code) >= 2   and length(code) <= 50),
	link_domain    varchar        not null default ''      check(link_domain = '' or (length(link_domain) >= 4 and length(link_domain) <= 255)),
	cname          varchar        null                     check(cname is null or (length(cname) >= 4 and length(cname) <= 255)),
	cname_setup_at timestamp      default null             check(cname_setup_at = strftime('%Y-%m-%d %H:%M:%S', cname_setup_at)),
	plan           varchar        not null                 check(plan in ('personal', 'personalplus', 'business', 'businessplus', 'child', 'custom')),
	stripe         varchar        null,
	billing_amount varchar,
	settings       varchar        not null,
	received_data  integer        not null default 0,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),
	first_hit_at   timestamp      not null                 check(first_hit_at = strftime('%Y-%m-%d %H:%M:%S', first_hit_at))
);
insert into sites2
	select id, parent, code, link_domain, cname, cname_setup_at, plan, stripe, billing_amount,
		settings, received_data, state, created_at, updated_at, created_at from sites;
drop table sites;
alter table sites2 rename to sites;
create unique index "sites#code" on sites(lower(code));
create index "sites#parent" on sites(parent) where state='a';
create unique index if not exists "sites#cname" on sites(lower(cname));


-- alter table users rename id   to user_id;
-- alter table users rename site to site_id;
-- alter table users drop constraint users_site_id_check;
create table users2 (
	user_id        integer        primary key autoincrement,
	site_id        integer        not null,

	email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
	email_verified integer        not null default 0,
	password       blob           default null,
	totp_enabled   integer        not null default 0,
	totp_secret    blob,
	role           varchar        not null default ''      check(role in ('', 'a')),
	login_at       timestamp      null                     check(login_at = strftime('%Y-%m-%d %H:%M:%S', login_at)),
	login_request  varchar        null,
	login_token    varchar        null,
	csrf_token     varchar        null,
	email_token    varchar        null,
	seen_updates_at timestamp     not null default current_timestamp check(seen_updates_at = strftime('%Y-%m-%d %H:%M:%S', seen_updates_at)),
	reset_at       timestamp      null,

	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict
);
insert into users2
	select id, site, email, email_verified, password, totp_enabled, totp_secret, role, login_at, login_request,
		login_token, csrf_token, email_token, seen_updates_at, reset_at, created_at, updated_at from users;
drop table users;
alter table users2 rename to users;
create        index "users#site_id"       on users(site_id);
create unique index "users#site_id#email" on users(site_id, lower(email));

-- alter table hits rename id to hit_id;
-- alter table hits rename site to site_id;
-- alter table hits drop column session;
-- alter table hits rename session2 to session;
-- alter table hits drop column path;
-- alter table hits drop column title;
-- alter table hits drop column event;
-- alter table hits drop column browser;
-- alter table hits add check(path_id > 0);
-- alter table hits add check(user_agent_id > 0);
create table hits2 (
	hit_id         integer        primary key autoincrement,
	site_id        integer        not null                 check(site_id > 0),
	path_id        integer        not null                 check(path_id > 0),
	user_agent_id  integer        not null                 check(user_agent_id > 0),

	session        blob           default null,
	bot            integer        default 0,
	ref            varchar        not null,
	ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
	size           varchar        not null default '',
	location       varchar        not null default '',
	first_visit    integer        default 0,

	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);
insert into hits2
	select id, site, path_id, user_agent_id, session2, bot, ref, ref_scheme, size, location, first_visit, created_at from hits;
drop table hits;
alter table hits2 rename to hits;
create index "hits#site_id#created_at" on hits(site_id, created_at);


-- alter table hit_counts rename site to site_id;
-- alter table hit_counts drop column path;
-- alter table hit_counts drop column title;
-- alter table hit_counts drop column event;
-- alter table hit_counts add foreign key (site_id) references sites(site_id) on delete restrict on update restrict;
drop table hit_counts;
create table hit_counts (
	site_id       integer    not null,
	path_id       integer    not null,

	hour          timestamp  not null check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
	total         integer    not null,
	total_unique  integer    not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "hit_counts#site_id#path_id#hour" unique(site_id, path_id, hour) on conflict replace
);
create index "hit_counts#site_id#hour" on hit_counts(site_id, hour);


-- alter table ref_counts     rename site to site_id;
-- alter table ref_counts drop column path;
-- alter table ref_counts add foreign key (site_id) references sites(site_id) on delete restrict on update restrict;
drop table ref_counts;
create table ref_counts (
	site_id       integer    not null,
	path_id       integer    not null,

	ref           varchar    not null,
	ref_scheme    varchar    null,
	hour          timestamp  not null check(hour=strftime('%Y-%m-%d %H:%M:%S', hour)),
	total         integer    not null,
	total_unique  integer    not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "ref_counts#site_id#path_id#ref#hour" unique(site_id, path_id, ref, hour) on conflict replace
);
create index "ref_counts#site_id#hour" on ref_counts(site_id, hour);


-- alter table hit_stats rename site to site_id;
-- alter table hit_stats drop column path;
-- alter table hit_stats drop column title;
-- alter table hit_stats drop constraint hit_stats_site_id_check;
drop table hit_stats;
create table hit_stats (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	stats          varchar        not null,
	stats_unique   varchar        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "hit_stats#site_id#path_id#day" unique(site_id, path_id, day) on conflict replace
);
create        index "hit_stats#site_id#day"         on hit_stats(site_id, day);


-- alter table browser_stats rename site to site_id;
-- alter table browser_stats drop column browser;
-- alter table browser_stats drop column version;
drop table browser_stats;
create table browser_stats (
	site_id        integer        not null,
	path_id        integer        not null,
	browser_id     integer        not null,

	day            date           not null                 check(day=strftime('%Y-%m-%d', day)),
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id)    references sites(site_id)       on delete restrict on update restrict,
	foreign key (browser_id) references browsers(browser_id) on delete restrict on update restrict
	constraint "browser_stats#site_id#path_id#day#browser_id" unique(site_id, path_id, day, browser_id) on conflict replace
);
create index "browser_stats#site_id#browser_id#day" on browser_stats(site_id, browser_id, day);


-- alter table system_stats rename site to site_id;
-- alter table system_stats drop column system;
-- alter table system_stats drop column version;
drop table system_stats;
create table system_stats (
	site_id        integer        not null,
	path_id        integer        not null,
	system_id      integer        not null,

	day            date           not null                 check(day=strftime('%Y-%m-%d', day)),
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id)   references sites(site_id)     on delete restrict on update restrict,
	foreign key (system_id) references systems(system_id) on delete restrict on update restrict
	constraint "system_stats#site_id#path_id#day#system_id" unique(site_id, path_id, day, system_id) on conflict replace
);
create index "system_stats#site_id#system_id#day" on system_stats(site_id, system_id, day);


-- alter table location_stats rename site to site_id;
drop table location_stats;
create table location_stats (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	location       varchar        not null,
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "location_stats#site_id#path_id#day#location" unique(site_id, path_id, day, location) on conflict replace
);
create index "location_stats#site_id#day" on location_stats(site_id, day);


-- alter table size_stats rename site to site_id;
drop table size_stats;
create table size_stats (
	site_id        integer        not null,
	path_id        integer        not null,

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	width          integer        not null,
	count          integer        not null,
	count_unique   integer        not null,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	constraint "size_stats#site_id#path_id#day#width" unique(site_id, path_id, day, width) on conflict replace
);
create index "size_stats#site_id#day" on size_stats(site_id, day);


-- Need to rename to FK on the following tables.

create table api_tokens2 (
	api_token_id   integer        primary key autoincrement,
	site_id        integer        not null,
	user_id        integer        not null,

	name           varchar        not null,
	token          varchar        not null                 check(length(token) > 10),
	permissions    varchar        not null,
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict,
	foreign key (user_id) references users(user_id) on delete restrict on update restrict
);
insert into api_tokens2
	select api_token_id, site_id, user_id, name, token, permissions, created_at from api_tokens;
drop table api_tokens;
alter table api_tokens2 rename to api_tokens;
create unique index "api_tokens#site_id#token" on api_tokens(site_id, token);

CREATE TABLE exports2 (
	export_id         integer        primary key autoincrement,
	site_id           integer        not null,
	start_from_hit_id integer        not null,

	path              varchar        not null,
	created_at        timestamp      not null    check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

	finished_at       timestamp                  check(finished_at is null or finished_at = strftime('%Y-%m-%d %H:%M:%S', finished_at)),
	last_hit_id       integer,
	num_rows          integer,
	size              varchar,
	hash              varchar,
	error             varchar,

	foreign key (site_id) references sites(site_id) on delete restrict on update restrict
);
insert into exports2
	select export_id, site_id, start_from_hit_id, path, created_at, finished_at, last_hit_id, num_rows, size, hash, error from exports;
drop table exports;
alter table exports2 rename to exports;
create index "exports#site_id#created_at" on exports(site_id, created_at);


CREATE TABLE paths2 (
		path_id        integer        primary key autoincrement,
		site_id        integer        not null,
		path           varchar        not null,
		title          varchar        not null default '',
		event          int            default 0,

		foreign key (site_id) references sites(site_id) on delete restrict on update restrict
	);
insert into paths2
	select path_id, site_id, path, title, event from paths;
drop table paths;
alter table paths2 rename to paths;
create unique index "paths#site_id#path" on paths(site_id, lower(path));
create        index "paths#path#title"   on paths(lower(path), lower(title));

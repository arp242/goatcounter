begin;
	-- alter table sites rename id to site_id;
	create table sites2 (
		site_id        integer        primary key autoincrement,
		parent         integer        null                     check(parent is null or parent>0),

		code           varchar        not null                 check(length(code) >= 2   and length(code) <= 50),
		link_domain    varchar        not null default ''      check(link_domain = '' or (length(link_domain) >= 4 and length(link_domain) <= 255)),
		cname          varchar        null                     check(cname is null or (length(cname) >= 4 and length(cname) <= 255)),
		cname_setup_at timestamp      default null             check(cname_setup_at = strftime('%Y-%m-%d %H:%M:%S', cname_setup_at)),
		plan           varchar        not null                 check(plan in ('personal', 'personalplus', 'business', 'businessplus', 'child', 'custom')),
		stripe         varchar        null,
		billing_amount varchar,
		settings       varchar        not null,
		received_data  int            not null default 0,

		state          varchar        not null default 'a'     check(state in ('a', 'd')),
		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
		updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at))
	);
	insert into sites2
		select id, parent, code, link_domain, cname, cname_setup_at, plan, stripe, billing_amount,
			settings, received_data, state, created_at, updated_at from sites;
	drop table sites;
	alter table sites2 rename to sites;
	create unique index "sites#code" on sites(lower(code));
	create index "sites#parent" on sites(parent) where state='a';
	create unique index if not exists "sites#cname" on sites(lower(cname));


	-- alter table users rename id   to user_id;
	-- alter table users rename site to site_id;
	create table users2 (
		user_id        integer        primary key autoincrement,
		site_id        integer        not null                 check(site_id > 0),

		email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
		email_verified int            not null default 0,
		password       blob           default null,
		totp_enabled   integer        not null default 0,
		totp_secret    blob,
		role           varchar        not null default ''      check(role in ('', 'a')),
		login_at       timestamp      null                     check(login_at = strftime('%Y-%m-%d %H:%M:%S', login_at)),
		login_request  varchar        null,
		login_token    varchar        null,
		csrf_token     varchar        null,
		email_token    varchar        null,
		seen_updates_at timestamp     not null default '1970-01-01 00:00:00' check(seen_updates_at = strftime('%Y-%m-%d %H:%M:%S', seen_updates_at)),
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
	create table hits2 (
		hit_id         integer        primary key autoincrement,
		site_id        integer        not null                 check(site_id > 0),
		session        blob           default null,

		bot            int            default 0,
		ref            varchar        not null,
		ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o', 'c')),
		size           varchar        not null default '',
		location       varchar        not null default '',
		first_visit    int            default 0,

		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),

		path_id        int            not null                 check(path_id > 0),
		user_agent_id  int            not null                 check(user_agent_id > 0)
	);
	insert into hits2
		select id, site, session2, bot, ref, ref_scheme, size, location, first_visit, created_at, path_id, user_agent_id from hits;
	drop table hits;
	alter table hits2 rename to hits;
	create index "hits#site_id#bot#created_at" on hits(site_id, bot, created_at);


	-- alter table hit_stats rename site to site_id;
	-- alter table hit_stats drop column path;
	-- alter table hit_stats drop column title;
	drop table hit_stats;
	create table hit_stats (
		site_id        integer        not null                 check(site_id > 0),
		path_id        int            not null                 check(path_id > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		stats          varchar        not null,
		stats_unique   varchar        not null,

		foreign key (site_id) references sites(site_id) on delete restrict on update restrict
	);
 	create unique index "hit_stats#site_id#path_id#day" on hit_stats(site_id, path_id, day);


	-- alter table hit_counts rename site to site_id;
	-- alter table hit_counts drop column path;
	-- alter table hit_counts drop column title;
	-- alter table hit_counts drop column event;
	drop table hit_counts;
	create table hit_counts (
		site_id       int        not null check(site_id>0),
		path_id       int        not null check(path_id > 0),

		hour          timestamp  not null check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
		total         int        not null,
		total_unique  int        not null,

		constraint "hit_counts#site_id#path_id#hour" unique(site_id, path_id, hour) on conflict replace
	);
	create index "hit_counts#site_id#hour" on hit_counts(site_id, hour);


	-- alter table ref_counts     rename site to site_id;
	-- alter table ref_counts drop column path;
	drop table ref_counts;
	create table ref_counts (
		site_id       int        not null check(site_id>0),
		path_id       int        not null check(path_id>0),

		ref           varchar    not null,
		ref_scheme    varchar    null,
		hour          timestamp  not null check(hour=strftime('%Y-%m-%d %H:%M:%S', hour)),
		total         int        not null,
		total_unique  int        not null,

		constraint "ref_counts#site_id#path_id#ref#hour" unique(site_id, path_id, ref, hour) on conflict replace
	);
	create index "ref_counts#site_id#hour" on ref_counts(site_id, hour);

	-- alter table browser_stats rename site to site_id;
	-- alter table browser_stats drop column browser;
	-- alter table browser_stats drop column version;
	drop table browser_stats;
	create table browser_stats (
		site_id        integer        not null                 check(site_id>0),
		path_id        integer        not null,
		browser_id     integer        not null,

		day            date           not null                 check(day=strftime('%Y-%m-%d', day)),
		count          int            not null,
		count_unique   int            not null,

		foreign key (site_id) references sites(site_id) on delete restrict on update restrict
	);
	create unique index "browser_stats#site_id#path_id#day#browser_id" on browser_stats(site_id, path_id, day, browser_id);
	create index "browser_stats#site_id#browser_id#day" on browser_stats(site_id, browser_id, day);


	-- alter table system_stats rename site to site_id;
	-- alter table system_stats drop column system;
	-- alter table system_stats drop column version;
	drop table system_stats;
	create table system_stats (
		site_id        integer        not null                 check(site_id>0),
		path_id        integer        not null,
		system_id      integer        not null,

		day            date           not null                 check(day=strftime('%Y-%m-%d', day)),
		count          int            not null,
		count_unique   int            not null,

		foreign key (site_id) references sites(site_id) on delete restrict on update restrict
	);
	create unique index "system_stats#site_id#path_id#day#system_id" on system_stats(site_id, path_id, day, system_id);
	create index "system_stats#site_id#system_id#day" on system_stats(site_id, system_id, day);


	-- alter table location_stats rename site to site_id;
	drop table location_stats;
	create table location_stats (
		site_id        integer        not null                 check(site_id > 0),
		path_id        integer        not null,

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		location       varchar        not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site_id) references sites(site_id) on delete restrict on update restrict
	);
	create unique index "location_stats#site_id#path_id#day#location" on location_stats(site_id, path_id, day, location);
    create index "location_stats#site_id#day" on location_stats(site_id, day);


	-- alter table size_stats rename site to site_id;
	drop table size_stats;
	create table size_stats (
		site_id        integer        not null                 check(site_id > 0),
		path_id        integer        not null,

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		width          int            not null,
		count          int            not null,
		count_unique   int            not null,

		foreign key (site_id) references sites(site_id) on delete restrict on update restrict
	);
	create unique index "size_stats#site_id#path_id#day#width" on size_stats(site_id, path_id, day, width);
    create index "size_stats#site_id#day" on size_stats(site_id, day);


 	insert into version values('2020-08-28-3-paths-rmold');
commit;

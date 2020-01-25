drop table if exists version;
create table version (name varchar);

drop table if exists sites cascade;
create table sites (
	id             serial         primary key,
	parent         integer        null                     check(parent is null or parent>0),

	name           varchar        not null                 check(length(name) >= 4 and length(name) <= 255),
	code           varchar        not null                 check(length(code) >= 2 and length(code) <= 50),
	cname          varchar        null                     check(cname is null or (length(cname) >= 4 and length(cname) <= 255)),
	plan           varchar        not null                 check(plan in ('p', 'b', 'e', 'c')),
	stripe         varchar        null,
	settings       varchar        not null,
	last_stat      timestamp      null,
	received_data  int            not null default 0,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null,
	updated_at     timestamp
);
create unique index "sites#code"  on sites(lower(code));
create unique index "sites#name"  on sites(lower(name));
create unique index "sites#cname" on sites(lower(cname));

drop table if exists users cascade;
create table users (
	id             serial         primary key,
	site           integer        not null                 check(site > 0),

	name           varchar        not null                 check(length(name) > 1  and length(name) <= 200),
	email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
	role           varchar        not null default ''      check(role in ('', 'a')),
	login_at       timestamp      null,
	login_request  varchar        null,
	login_token    varchar        null,
	csrf_token     varchar        null,

	created_at     timestamp      not null,
	updated_at     timestamp,

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create unique index "users#login_request" on users(login_request);
create unique index "users#login_token"   on users(login_token);
create        index "users#site"          on users(site);
create unique index "users#site#email"    on users(site, lower(email));

drop table if exists hits cascade;
create table hits (
	site           integer        not null                 check(site > 0),

	path           varchar        not null,
	ref            varchar        not null,
	ref_original   varchar,
	ref_params     varchar,
	ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o')),
	browser        varchar        not null,
	size           varchar        not null default '',

	created_at     timestamp      not null
);
create index "hits#site#created_at"      on hits(site, created_at);
create index "hits#site#path#created_at" on hits(site, lower(path), created_at);

drop table if exists hit_stats cascade;
create table hit_stats (
	site           integer        not null                 check(site > 0),

	day            date           not null,
	path           varchar        not null,
	stats          varchar        not null,

	created_at     timestamp      null,
	updated_at     timestamp,

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create index "hit_stats#site#day" on hit_stats(site, day);

drop table if exists browser_stats cascade;
create table browser_stats (
	site           integer        not null                 check(site > 0),

	day            date           not null,
	browser        varchar        not null,
	version        varchar        not null,
	count          int            not null,
	mobile         int default 0  not null,

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create index "browser_stats#site#day"         on browser_stats(site, day);
create index "browser_stats#site#day#browser" on browser_stats(site, day, browser);

drop table if exists version;
create table version (
	name varchar
);
insert into version values ('0000-00-00 00:00:00 init');

drop table if exists sites;
create table sites (
	id             integer        primary key autoincrement,

	code           varchar        not null                 check(length(code) >= 2   and length(code) <= 50),
	plan           varchar        not null                 check(plan in ('p', 'b', 'e')),
	stripe         varchar        null,
	settings       varchar        not null,
	last_stat      timestamp      null                     check(last_stat = strftime('%Y-%m-%d %H:%M:%S', last_stat)),
	received_data  int            not null default 0,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at))
);
create unique index "sites#code" on sites(lower(code));

drop table if exists domains;
create table domains (
	id             integer        primary key autoincrement,
	site           integer        not null                 check(site > 0),

	domain         varchar        not null                 check(length(domain) >= 4 and length(domain) <= 255),
	display_order  integer        not null default 0,
	bg_color       varchar        null,
	color          varchar        null,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create index        "domains#site"        on domains(site);
create unique index "domains#site#domain" on domains(site, lower(domain));

drop table if exists users;
create table users (
	id             integer        primary key autoincrement,
	site           integer        not null                 check(site > 0),

	name           varchar        not null                 check(length(name) > 1  and length(name) <= 200),
	email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
	role           varchar        not null default ''      check(role in ('', 'a')),
	login_req      timestamp      null                     check(login_req = strftime('%Y-%m-%d %H:%M:%S', login_req)),
	login_key      varchar        null,
	csrf_token     varchar        null,
	preferences    varchar        not null default '{}',

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create unique index "users#login_key"  on users(login_key);
create        index "users#site"       on users(site);
create unique index "users#site#email" on users(site, lower(email));

drop table if exists hits;
create table hits (
	-- No foreign key on site for performance.
	site           integer        not null                 check(site > 0),

	path           varchar        not null,
	ref            varchar        not null,
	ref_original   varchar,
	ref_params     varchar,
	ref_scheme     varchar        null                     check(ref_scheme in ('h', 'g', 'o')),

	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);
create index "hits#site#created_at"      on hits(site, created_at);
create index "hits#site#path#created_at" on hits(site, lower(path), created_at);

drop table if exists hit_stats;
create table hit_stats (
	site           integer        not null                 check(site > 0),

	day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
	path           varchar        not null,
	stats          varchar        not null,

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create index "hit_stats#site#day" on hit_stats(site, day);

drop table if exists browsers;
create table browsers (
	-- No foreign key on site for performance.
	site           integer        not null                 check(site > 0),

	browser        varchar        not null,
	created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);
create index "browsers#site#created_at" on browsers(site, created_at);

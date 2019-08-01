drop table if exists version;
create table version (
	name varchar
);
insert into version values ('0000-00-00 00:00:00 init');

drop table if exists sites;
create table sites (
	id             integer        primary key autoincrement,

	domain         varchar        not null collate nocase  check(length(domain) <= 255),
	code           varchar        not null collate nocase  check(length(domain) <= 50),
	plan           varchar        not null                 check(plan in ('p', 'b', 'e')),
	settings       varchar        not null,
	last_stat      datetime       null                     check(last_stat = strftime('%Y-%m-%d %H:%M:%S', last_stat)),
	received_data  int            not null default 0,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     datetime       not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     datetime                                check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at))
);
create unique index 'sites#code'   on sites(code);
create unique index 'sites#domain' on sites(domain);

drop table if exists users;
create table users (
	id             integer        primary key autoincrement,
	site           integer        not null                 check(site > 0),

	name           varchar        not null                 check(length(name) <= 200),
	email          varchar        not null collate nocase  check(length(email) <= 255),
	role           varchar        not null default ''      check(role in ('', 'a')),
	login_req      datetime       null                     check(login_req = strftime('%Y-%m-%d %H:%M:%S', login_req)),
	login_key      varchar        null,
	csrf_token     varchar        null,
	preferences    varchar        not null default '{}',

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     datetime       not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     datetime                                check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create unique index 'users#login_key'  on users(login_key);
create        index 'users#site'       on users(site);
create unique index 'users#site#email' on users(site, email);

drop table if exists hits;
create table hits (
	-- No foreign key on site for performance.
	site           integer        not null                 check(site > 0),

	path           varchar        not null collate nocase,
	ref            varchar        not null collate nocase,
	ref_original   varchar,
	ref_params     varchar,

	created_at     datetime       not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);
create index 'hits#site#created_at'      on hits(site, created_at);
create index 'hits#site#path#created_at' on hits(site, path, created_at);

drop table if exists hit_stats;
create table hit_stats (
	site           integer        not null                 check(site > 0),

	day            date           not null,
	path           varchar        not null collate nocase,
	stats          varchar        not null,

	created_at     datetime       null                     check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
	updated_at     datetime                                check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at)),

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create index 'hit_stats#site#day'      on hit_stats(site, day);

drop table if exists browsers;
create table browsers (
	-- No foreign key on site for performance.
	site           integer        not null                 check(site > 0),

	browser        varchar        not null collate nocase,
	created_at     datetime       not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at))
);
create index 'browsers#site#created_at' on browsers(site, created_at);

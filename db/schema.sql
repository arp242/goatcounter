-- TODO(v1): deal with casing; this should work: where email = "MARTIN@ARP242.NET"
-- TODO(v1): validate the dates
-- TODO(v1): test postgresql
-- check(strftime('%Y-%m-%d', created_at) = created_at)

drop table if exists version;
create table version (
	name varchar
);
-- TODO(v1): check on startup and refuse to run if out of date.
insert into version values ("2019-06-29 init");

drop table if exists sites;
create table sites (
	id             integer        primary key autoincrement,

	domain         varchar        not null unique check(length(domain) <= 255),
	code           varchar        not null unique check(length(domain) <= 50),
	settings       varchar        not null default "{}",
	last_stat      datetime       null,               -- When stats ran last.
	received_data  int            not null default 0, -- Set when we receive one event.

	state          varchar        not null default "a" check(state in ("a", "d")),
	created_at     datetime       not null default current_timestamp,
	updated_at     datetime
);

drop table if exists users;
create table users (
	id             integer        primary key autoincrement,
	site           integer        not null check(site > 0),

	name           varchar        not null check(length(name) <= 200),
	email          varchar        not null check(length(email) <= 255),
	role           varchar        not null default "" check(role in ("", "a")),
	-- TODO(v1): the login_req and login_key should be in a new table, so we can
	-- support multiple logins per user.
	login_req      datetime       null,
	login_key      varchar        null unique,
	csrf_token     varchar        null,
	preferences    varchar        not null default "{}",

	state          varchar        not null default "a" check(state in ("a", "d")),
	created_at     datetime       not null default current_timestamp,
	updated_at     datetime,

	unique(email, site),
	foreign key (site) references sites(id)
);

drop table if exists hits;
create table hits (
	site           integer        not null check(site > 0),

	path           varchar        not null,
	ref            varchar        not null,
	ref_original   varchar,
	ref_params     varchar,

	created_at     datetime       null default current_timestamp

	-- no fkey for performance
	-- foreign key (site) references sites(id)
);

drop table if exists hit_stats;
create table hit_stats (
	site           integer        not null check(site > 0),

	kind           varchar        not null check(kind in ("h", "d")), -- hourly, daily
	day            date           not null,  -- "2019-06-22"
	path           varchar        not null,  -- /foo.html
	stats          varchar        not null,  -- hourly or daily hits [20, 30, ...]

	created_at     datetime       null default current_timestamp,
	updated_at     datetime

	-- no fkey for performance
	--unique(site, kind, day, path)
	--foreign key (site) references sites(id)
);

drop table if exists browser_stats;
create table browser_stats (
	site           integer        not null check(site > 0),

	browser        varchar        not null,
	created_at     datetime       not null default current_timestamp

	-- no fkey for performance
	--foreign key (site) references sites(id)
);

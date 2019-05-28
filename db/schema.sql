-- TODO: deal with casing; this should work: where email = "MARTIN@ARP242.NET"
-- TODO: validate the dates
-- check(strftime('%Y-%m-%d', created_at) = created_at)

drop table if exists sites;
create table sites (
	id             integer        primary key autoincrement,

	name           varchar        not null check(length(name) <= 100),
	domain         varchar        not null unique check(length(domain) <= 255),
	code           varchar        not null unique check(length(domain) <= 50),

	state          varchar        not null default "a" check(state in ("a", "d")),
	created_at     datetime       not null default current_timestamp,
	updated_at     datetime
);
insert into sites (domain, code, name) values
	("arp242.net",     "arp242",    "arp242.net"),
	("zgo.at",         "zgoat",      "zGoat"),
	("goatletter.com", "goatletter", "GoatLetter");

drop table if exists users;
create table users (
	id             integer        primary key autoincrement,
	site           integer        not null check(site > 0),

	name           varchar        not null check(length(name) <= 200),
	email          varchar        not null check(length(email) <= 255),
	role           varchar        not null default "" check(role in ("", "a")),
	login_req      datetime       null,
	login_key      varchar        null unique,
	csrf_token     varchar        null,

	state          varchar        not null default "a" check(state in ("a", "d")),
	created_at     datetime       not null default current_timestamp,
	updated_at     datetime,

	unique(email, site)
	foreign key (site) references sites(id)
);
insert into users (site, name, email, role) values
	(1, "Martin Tournoij", "martin@arp242.net", "a");

-- List of all subscribed emails.
drop table if exists hits;
create table hits (
	--id             integer        primary key autoincrement,
	site           integer        not null check(site > 0),

	path           varchar        not null,
	ref            varchar        not null,

	created_at     datetime       null default current_timestamp,

	foreign key (site) references sites(id)
);

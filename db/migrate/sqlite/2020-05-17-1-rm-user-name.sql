begin;
	insert into version values ('2020-05-17-1-rm-user-name');

	create table users2 (
		id             integer        primary key autoincrement,
		site           integer        not null                 check(site > 0),

		password       blob           default null,
		email          varchar        not null                 check(length(email) > 5 and length(email) <= 255),
		email_verified int            not null default 0,
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

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into users2 (
		id, site, password, email, email_verified, role, login_at, login_request, login_token, csrf_token, email_token, seen_updates_at, reset_at, created_at, updated_at
	) select
		id, site, password, email, email_verified, role, login_at, login_request, login_token, csrf_token, email_token, seen_updates_at, reset_at, created_at, updated_at
	from users;
	drop table users;
	alter table users2 rename to users;
	create unique index "users#login_request" on users(login_request);
	create unique index "users#login_token"   on users(login_token);
	create        index "users#site"          on users(site);
	create unique index "users#site#email"    on users(site, lower(email));

	create table sites2 (
		id             integer        primary key autoincrement,
		parent         integer        null                     check(parent is null or parent>0),

		code           varchar        not null                 check(length(code) >= 2   and length(code) <= 50),
		link_domain    varchar        not null default ''      check(link_domain = '' or (length(link_domain) >= 4 and length(link_domain) <= 255)),
		cname          varchar        null                     check(cname is null or (length(cname) >= 4 and length(cname) <= 255)),
		plan           varchar        not null                 check(plan in ('personal', 'personalplus', 'business', 'businessplus', 'child', 'custom')),
		stripe         varchar        null,
		settings       varchar        not null,
		received_data  int            not null default 0,

		state          varchar        not null default 'a'     check(state in ('a', 'd')),
		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
		updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at))
	);
	insert into sites2 (
		id, parent, code, link_domain, cname, plan, stripe, settings, received_data, state, created_at, updated_at
	) select
		id, parent, code, link_domain, cname, plan, stripe, settings, received_data, state, created_at, updated_at
	from sites;
	drop table sites;
	alter table sites2 rename to sites;

	create unique index "sites#code" on sites(lower(code));

	insert into version values ('2020-05-17-1-rm-user-name');
commit;

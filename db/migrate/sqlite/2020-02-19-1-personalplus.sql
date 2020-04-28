begin;
	create table sites2 (
		id             integer        primary key autoincrement,
		parent         integer        null                     check(parent is null or parent>0),

		name           varchar        not null                 check(length(name) >= 4 and length(name) <= 255),
		code           varchar        not null                 check(length(code) >= 2   and length(code) <= 50),
		cname          varchar        null                     check(cname is null or (length(cname) >= 4 and length(cname) <= 255)),
		plan           varchar        not null                 check(plan in ('personal', 'personalplus', 'business', 'businessplus', 'child', 'custom')),
		stripe         varchar        null,
		settings       varchar        not null,
		last_stat      timestamp      null                     check(last_stat = strftime('%Y-%m-%d %H:%M:%S', last_stat)),
		received_data  int            not null default 0,
		link_domain    varchar        not null default '',

		state          varchar        not null default 'a'     check(state in ('a', 'd')),
		created_at     timestamp      not null                 check(created_at = strftime('%Y-%m-%d %H:%M:%S', created_at)),
		updated_at     timestamp                               check(updated_at = strftime('%Y-%m-%d %H:%M:%S', updated_at))
	);

	insert into sites2 select
		id, parent, name, code, cname, plan, stripe, settings, last_stat, received_data, link_domain, state, created_at, updated_at
	from sites;
	drop table sites;
	alter table sites2 rename to sites;

	insert into version values ('2020-02-19-1-personalplus');
commit;

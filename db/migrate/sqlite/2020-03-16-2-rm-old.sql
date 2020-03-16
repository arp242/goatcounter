begin;
	--alter table hit_stats drop column total;
	create table hit_stats2 (
		site           integer        not null                 check(site > 0),

		day            date           not null                 check(day = strftime('%Y-%m-%d', day)),
		path           varchar        not null,
		title          varchar        not null default '',
		stats          varchar        not null,

		foreign key (site) references sites(id) on delete restrict on update restrict
	);
	insert into hit_stats2
		(site, day, path, title, stats)
		select site, day, path, title, stats from hit_stats;
	drop table hit_stats;
	alter table hit_stats2 rename to hit_stats;

	--alter table sites     drop column last_stat;
	--alter table sites
	--	drop constraint if exists sites_domain_check;
	--alter table sites
	--	add constraint sites_link_domain_check check(length(link_domain) >= 4 and length(link_domain) <= 255);
	create table sites2 (
		id             integer        primary key autoincrement,
		parent         integer        null                     check(parent is null or parent>0),

		name           varchar        not null                 check(length(name) >= 4 and length(name) <= 255),
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
	insert into sites2
		select id, parent, name, code, link_domain, cname, plan, stripe, settings, received_data, state, created_at, updated_at from sites;
	drop table sites;
	alter table sites2 rename to sites;

	insert into version values ('2020-03-16-2-rm-old');
commit;

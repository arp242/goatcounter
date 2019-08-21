-- Was never used, might as well drop here.
alter table hit_stats drop created_at;
alter table hit_stats drop updated_at;

-- Create domains table.
create table domains (
	id             serial         primary key,
	site           integer        not null                 check(site > 0),

	domain         varchar        not null                 check(length(domain) >= 4 and length(domain) <= 255),
	display_order  integer        not null default 0,
	bg_color       varchar        null,
	color          varchar        null,

	state          varchar        not null default 'a'     check(state in ('a', 'd')),
	created_at     timestamp      not null,
	updated_at     timestamp,

	foreign key (site) references sites(id) on delete restrict on update restrict
);
create index        "domains#site"        on domains(site);
create unique index "domains#site#domain" on domains(site, lower(domain));

-- Link hits and browsers to a domain, rather than site.
alter table hits      add column domain integer not null default 1 check(domain > 0);
alter table hit_stats add column domain integer not null default 1 check(domain > 0);
alter table browsers  add column domain integer not null default 1 check(domain > 0);

-- Populate with correct domains.
insert into domains (site, domain, created_at) select id, domain, created_at from sites;

update hits      set domain=(select id from sites where id=hits.site);
update hit_stats set domain=(select id from sites where id=hit_stats.site);
update browsers  set domain=(select id from sites where id=browsers.site);

-- Drop old domain column for site.
alter table sites drop column domain;

-- Remove temporary default values now that everything is filled in.
alter table hits      alter domain drop default;
alter table hit_stats alter domain drop default;
alter table domains   alter domain drop default;

-- Merge sites

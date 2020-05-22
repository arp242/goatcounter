begin;
	create table botlog_ips (
		id             serial         primary key,

		count          int            not null default 1,
		ip             varchar        not null,
		ptr            varchar,
		info           varchar,
		hide           int            default 0,

		created_at     timestamp      not null,
		last_seen      timestamp      not null
	);
	create unique index "botlog_ips#ip" on botlog_ips(ip);

	create table botlog (
		id             serial         primary key,

		ip             int            not null,
		bot            int            not null,
		ua             varchar        not null,
		headers        jsonb          not null,
		url            varchar        not null,
		created_at     timestamp      not null,

		foreign key (ip) references botlog_ips(id)
	);

	insert into version values ('2020-05-23-1-botlog');
commit;

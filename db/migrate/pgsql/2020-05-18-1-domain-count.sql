begin;
	create table hit_counts (
		site          int        not null check(site>0),
		path          varchar    not null,
		title         varchar    not null,
		event         integer    not null default 0,
		hour          timestamp  not null,
		total         int        not null,
		total_unique  int        not null,

		constraint "hit_counts#site#path#hour#event" unique(site, path, hour, event)
	);
	create index "hit_counts#site#hour#event" on hit_counts(site, hour, event);

	insert into hit_counts (site, path, title, event, hour, total, total_unique)
		select
				site,
				max(path),
				max(title) as title,
				event,
				(substring(created_at::varchar, 0, 14) || ':00:00')::timestamp as hour,
				count(*),
				sum(first_visit)
		from hits
		where bot=0
		group by site, lower(path), event, hour;

	insert into version values ('2020-05-18-1-domain-count');
commit;

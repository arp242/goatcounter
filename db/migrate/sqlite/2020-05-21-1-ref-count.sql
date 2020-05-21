begin;
	create table ref_counts (
		site          int        not null check(site>0),
		path          varchar    not null,
		ref           varchar    not null,
		ref_scheme    varchar    null,
		hour          timestamp  not null check(hour = strftime('%Y-%m-%d %H:%M:%S', hour)),
		total         int        not null,
		total_unique  int        not null,

		constraint "ref_counts#site#path#ref#hour" unique(site, path, ref, hour) on conflict replace
	);

	insert into ref_counts (site, path, ref, ref_scheme, hour, total, total_unique)
		select
				site,
				max(path),
				max(ref) as ref,
				max(ref_scheme),
				(substr(created_at, 0, 14) || ':00:00') as hour,
				count(*),
				sum(first_visit)
		from hits
		where bot=0
		group by site, lower(path), ref, hour;

	create index "ref_counts#site#hour" on ref_counts(site, hour);

	insert into version values ('2020-05-21-1-ref-count');
commit;

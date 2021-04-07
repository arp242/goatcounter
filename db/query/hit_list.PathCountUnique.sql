with x as (
	select path_id, path from paths
	where site_id = :site and lower(path) = lower(:path)
)
select
	path,
	(
		select coalesce(sum(total_unique), 0)
		from hit_counts where
			site_id = :site and
			path_id = x.path_id
			{{:start and hour >= :start}}
			{{:end   and hour <= :end}}
	) as count_unique
from x
group by path, path_id

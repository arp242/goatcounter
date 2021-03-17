with x as (
	select path_id, path from paths
	where site_id = :site and lower(path) = lower(:path)
)
select
	path,
	coalesce(sum(
		(select sum(total_unique) from hit_counts where site_id = :site and path_id = x.path_id)
	), 0) as count_unique
from x
group by path

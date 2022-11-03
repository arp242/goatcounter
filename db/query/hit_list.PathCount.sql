with x as (
	select path_id, path from paths
	where site_id = :site and lower(path) = lower(:path)
)
select
	x.path,
	coalesce(sum(total), 0) as count
from hit_counts
join x using (path_id)
where
	site_id = :site and
	path_id = x.path_id
	{{:start and hour >= :start}}
	{{:end   and hour <= :end}}
group by x.path, hit_counts.path_id

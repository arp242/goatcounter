with x as (
	select ref_id, ref_scheme from refs
	where lower(ref) = lower(:ref)
	limit :limit offset :offset
),
y as (
	select
		path_id,
		coalesce(sum(total), 0) as count
	from ref_counts
	join x using (ref_id)
	where site_id = :site and hour >= :start and hour <= :end and :filter
	group by path_id
	order by count desc
)
select
	paths.path as name,
	y.count
from y
join paths using(path_id)
order by count desc

with x as (
	select
		path_id,
		coalesce(sum(total), 0) as count
	from ref_counts
	where
		site_id = :site and hour >= :start and hour <= :end and
		{{:filter path_id in (:filter) and}}
		ref = :ref
	group by path_id
	order by count desc
	limit :limit offset :offset
)
select
	paths.path as name,
	x.count
from x
join paths using(path_id)
order by count desc

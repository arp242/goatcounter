with x as (
	select
		path_id,
		coalesce(sum(total), 0)        as count,
		coalesce(sum(total_unique), 0) as count_unique
	from ref_counts
	where
		site_id = :site and hour >= :start and hour <= :end and
		{{:filter path_id in (:filter) and}}
		ref = :ref
	group by path_id
	order by count_unique desc
	limit :limit offset :offset
)
select
	paths.path as name,
	x.count,
	x.count_unique
from x
join paths using(path_id)

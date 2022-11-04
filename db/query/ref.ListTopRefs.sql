with x as (
	select
        coalesce(ref_id, 1)     as ref_id,
		coalesce(sum(total), 0) as count
	from ref_counts
	where
		site_id = :site and hour >= :start and hour <= :end
		{{:filter and path_id in (:filter)}}
	group by ref_id
	order by count desc, ref_id
	-- Over-select quite a bit here since we may filter on the refs.ref below;
	-- even with the over-select a CTE is quite a bit faster.
	limit :limit2 offset :offset
)
select
	x.count,
	refs.ref_scheme as ref_scheme,
	refs.ref        as name
from x
left join refs using (ref_id)
{{:has_domain where refs.ref not like :ref}}
limit :limit

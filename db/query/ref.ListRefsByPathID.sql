with x as (
	select
		ref_id,
		coalesce(sum(total), 0) as count
	from ref_counts
	where
		site_id = :site and path_id = :path and hour >= :start and hour <= :end
	group by ref_id
	order by count desc, ref_id
	limit :limit offset :offset
)
select
	x.count,
	refs.ref_scheme as ref_scheme,
	-- This coalesce shouldn't be needed, and is just here to fix a borked
	-- migration on my own site until such time I can properly fix this.
	coalesce(refs.ref, '') as name
from x
left join refs using (ref_id)
order by count desc, ref_id

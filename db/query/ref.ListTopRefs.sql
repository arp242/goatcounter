select
	coalesce(sum(total), 0)        as count,
	coalesce(sum(total_unique), 0) as count_unique,
	max(ref_scheme)                as ref_scheme,
	ref                            as name
from ref_counts
where
	site_id = :site and hour >= :start and hour <= :end
	{{:filter     and path_id in (:filter)}}
	{{:has_domain and ref not like :ref}}
group by ref
order by count_unique desc, ref
limit 6 offset :offset

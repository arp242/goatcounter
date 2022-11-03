select
	coalesce(sum(total), 0) as count,
	max(ref_scheme)                as ref_scheme,
	ref                            as name
from ref_counts
where
	site_id = :site and hour >= :start and hour <= :end
	{{:filter     and path_id in (:filter)}}
	{{:has_domain and ref not like :ref}}
group by ref
order by count desc, ref
limit :limit offset :offset

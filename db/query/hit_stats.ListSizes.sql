select
	width             as name,
	sum(count_unique) as count_unique
from size_stats
where
	site_id = :site and day >= :start and day <= :end
	{{:filter and path_id in (:filter)}}
group by width
order by count_unique desc, name asc

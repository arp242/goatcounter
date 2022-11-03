select
	'↔ ' || width || 'px' as name,
	sum(count)     as count
from size_stats
where
	site_id = :site and day >= :start and day <= :end
	{{:filter   and path_id in (:filter)}}
	{{:max_size and width != 0 and width > :min_size and width <= :max_size}}
	{{:empty    and width = 0}}
group by width
order by count desc, name asc
limit :limit offset :offset

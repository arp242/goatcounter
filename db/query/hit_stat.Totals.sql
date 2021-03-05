select hour, sum(total) as total, sum(total_unique) as total_unique
from hit_counts
{{:no_events join paths using (path_id)}}
where
	hit_counts.site_id = :site and hour >= :start and hour <= :end
	{{:no_events and paths.event = 0}}
	{{:filter and path_id in (:filter)}}
group by hour
order by hour asc

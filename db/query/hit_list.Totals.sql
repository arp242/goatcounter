with x as (
	select
		sum(total) as total,
		{{:sqlite  substr(datetime(hour, :offset2), 0, 14)                     as hour}}
		{{:sqlite! substr((hour + :offset * interval '1 minute')::text, 0, 14) as hour}}
	from hit_counts
	{{:no_events join paths using (path_id)}}
	where
		hit_counts.site_id = :site and hour >= :start and hour <= :end and
		{{:no_events paths.event = 0 and}}
		:filter
	group by hour
	order by hour asc
)
select
	{{:sqlite  json_group_object(hour, total) as stats2}}
	{{:sqlite! jsonb_object_agg(hour, total)  as stats2}}
from x

with x as (
	select
		sum(total) as total,
		path_id,
		{{:sqlite  json_group_object(substr(datetime(hour, :offset2), 0, 14), total) as stats2}}
		{{:sqlite! jsonb_object_agg(substr((hour + :offset * interval '1 minute')::text, 0, 14), total) as stats2}}
	from hit_counts
	where
		hit_counts.site_id = :site and
		{{:exclude not path_id :in (:exclude) and}}
		:filter and
		hour >=:start and hour<=:end
	group by path_id
	order by total desc, path_id desc
	limit :limit
)
select
	path_id,
	paths.path,
	paths.title,
	paths.event,
	total as count,
	stats2
from x
join paths using (path_id)
order by total desc, path_id desc

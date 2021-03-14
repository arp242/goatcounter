with x as (
	select
		system_id,
		sum(count)        as count,
		sum(count_unique) as count_unique
	from system_stats
	where
		site_id = :site and day >= :start and day <= :end
		{{:filter and path_id in (:filter)}}
	group by system_id
	order by count_unique desc
)
select
	systems.name,
	sum(x.count)        as count,
	sum(x.count_unique) as count_unique
from x
join systems using (system_id)
group by systems.name
order by count_unique desc
limit :limit offset :offset

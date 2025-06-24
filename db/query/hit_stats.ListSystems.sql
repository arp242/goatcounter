with x as (
	select
		system_id,
		sum(count) as count
	from system_stats
	where
		site_id = :site and day >= :start and day <= :end
		{{:filter and path_id :in (:filter)}}
	group by system_id
	order by count desc
)
select
	systems.name,
	sum(x.count) as count
from x
join systems using (system_id)
group by systems.name
order by count desc
limit :limit offset :offset

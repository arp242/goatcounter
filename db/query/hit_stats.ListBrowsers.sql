with x as (
	select
		browser_id,
		sum(count_unique) as count_unique
	from browser_stats
	where
		site_id = :site and day >= :start and day <= :end
		{{:filter and path_id in (:filter)}}
	group by browser_id
	order by count_unique desc
)
select
	browsers.name,
	sum(x.count_unique) as count_unique
from x
join browsers using (browser_id)
group by browsers.name
order by count_unique desc
limit :limit offset :offset

with x as (
	select
		browser_id,
		sum(count) as count
	from browser_stats
	where site_id = :site and day >= :start and day <= :end and :filter
	group by browser_id
	order by count desc
)
select
	browsers.name,
	sum(x.count) as count
from x
join browsers using (browser_id)
group by browsers.name
order by count desc
limit :limit offset :offset

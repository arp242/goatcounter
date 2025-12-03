with x as (
	select
		substr(location, 0, 3) as loc,
		sum(count)      as count
	from location_stats
	where site_id = :site and day >= :start and day <= :end and :filter
	group by loc
	order by count desc, loc
	limit :limit offset :offset
)
select
	locations.iso_3166_2   as id,
	locations.country_name as name,
	x.count         as count
from x
join locations on locations.iso_3166_2 = x.loc
order by count desc, name asc

with x as (
	select
		substr(location, 0, 3) as loc,
		sum(count)             as count,
		sum(count_unique)      as count_unique
	from location_stats
	where
		site_id = :site and day >= :start and day <= :end
		{{:filter and path_id in (:filter)}}
	group by loc
	order by count_unique desc, loc
	limit :limit offset :offset
)
select
	locations.iso_3166_2   as id,
	locations.country_name as name,
	x.count                as count,
	x.count_unique         as count_unique
from x
join locations on locations.iso_3166_2 = x.loc
order by count_unique desc, name asc

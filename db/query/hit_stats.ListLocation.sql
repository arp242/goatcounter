select
	coalesce(region_name, '(unknown)') as name,
	sum(count_unique)                  as count_unique
from location_stats
join locations on location = iso_3166_2
where
	site_id = :site and day >= :start and day <= :end and
	{{:filter path_id in (:filter) and}}
	country = :country
group by iso_3166_2, name
order by count_unique desc, name asc
limit :limit offset :offset

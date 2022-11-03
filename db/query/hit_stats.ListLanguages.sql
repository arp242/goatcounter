with x as (
	select
		language,
		sum(count) as count
	from language_stats
	where
		site_id = :site and day >= :start and day <= :end
		{{:filter and path_id in (:filter)}}
	group by language
	order by count desc, language
	limit :limit offset :offset
)
select
	languages.iso_639_3 as id,
	languages.name      as name,
	x.count             as count
from x
join languages on languages.iso_639_3 = x.language
order by count desc, name asc

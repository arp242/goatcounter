with x as (
	select
		campaign_id,
		sum(count)        as count,
		sum(count_unique) as count_unique
	from campaign_stats
	where
		site_id = :site and day >= :start and day <= :end
		{{:filter and path_id in (:filter)}}
	group by campaign_id
	order by count_unique desc, campaign_id
	limit :limit offset :offset
)
select
	campaign_id     as id,
	campaigns.name  as name,
	x.count         as count,
	x.count_unique  as count_unique
from x
join campaigns using (campaign_id)
order by count_unique desc, name asc

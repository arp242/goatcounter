with x as (
	select
		campaign_id,
		sum(count) as count
	from campaign_stats
	where site_id = :site and day >= :start and day <= :end and :filter
	group by campaign_id
	order by count desc, campaign_id
	limit :limit offset :offset
)
select
	campaign_id     as id,
	campaigns.name  as name,
	x.count  as count
from x
join campaigns using (campaign_id)
order by count desc, name asc

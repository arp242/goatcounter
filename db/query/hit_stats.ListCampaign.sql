select
	ref               as name,
	sum(count) as count
from campaign_stats
join campaigns using (campaign_id)
where
	campaign_stats.site_id = :site and day >= :start and day <= :end and
	:filter and
	campaign_id = :campaign
group by campaign_id, ref
order by count desc, ref asc
limit :limit offset :offset

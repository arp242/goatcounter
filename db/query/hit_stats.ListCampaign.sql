select
	ref               as name,
	sum(count_unique) as count_unique
from campaign_stats
join campaigns using (campaign_id)
where
	campaign_stats.site_id = :site and day >= :start and day <= :end and
	{{:filter path_id in (:filter) and}}
	campaign_id = :campaign
group by campaign_id, ref
order by count_unique desc, ref asc
limit :limit offset :offset

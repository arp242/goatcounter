with prev as (
	select
		path_id,
		sum(total) as total
	from hit_counts
	where
		site_id = :site and path_id :in (:paths) and
		hour >= :prevstart and hour <= :prevend
	group by path_id
),
cur as (
	select
		path_id,
		sum(c.total) as total
	from hit_counts c
	where
		site_id = :site and path_id :in (:paths) and
		hour >= :start and hour <= :end
	group by path_id
)
select
	percent_diff(coalesce(prev.total, 0), coalesce(cur.total, 0)) as diff
from cur
left join prev using (path_id)
order by cur.total desc, path_id desc

select path_id, day, stats
from hit_stats
where
	hit_stats.site_id = :site and
	path_id :in (:paths) and
	day >= :start and day <= :end
order by day asc

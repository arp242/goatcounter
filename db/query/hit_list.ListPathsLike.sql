with x as (
	select path_id, path, title from paths
	where site_id = :site and
	(lower(path) like lower(:search) {{:match_title or lower(title) like lower(:search)}})
)
select
	path_id, path, title,
	sum(total) as count
from hit_counts
join x using(path_id)
where site_id = :site
group by path_id, path, title
order by count desc

with x as (
	select sum(total) as total, path_id from hit_counts
	where
		hit_counts.site_id = :site and
		{{:exclude not path_id :in (:exclude) and}}
		{{:filter path_id :in (:filter) and}}
		hour>=:start and hour<=:end
	group by path_id
	order by total desc, path_id desc
	limit :limit
)
select path_id, paths.path, paths.title, paths.host, paths.event from x
join paths using (path_id)
order by total desc, path_id desc

select coalesce(max(total_unique), 0) from hit_counts
where
	hit_counts.site_id = :site and hour >= :start and hour <= :end
	{{:filter and path_id in (:filter)}}
